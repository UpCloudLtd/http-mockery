package mockery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/valyala/fasttemplate"
)

var ErrEndpointNotFound = fmt.Errorf("no matching endpoint found")

var validHTTPMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

const (
	EndpointTypeNormal = "normal"
	EndpointTypeRegex  = "regex"

	ContextKeyReqBody = "requestBody"
	ContextKeyResBody = "responseBody"
)

var validEndpointTypes = []string{EndpointTypeNormal, EndpointTypeRegex}

type Config struct {
	ListenIP   string     `json:"listen_ip"`
	ListenPort int        `json:"listen_port"`
	ProxyPass  string     `json:"proxy_pass"`
	Endpoints  []Endpoint `json:"endpoints"`
	Logging    Logging    `json:"logging"`
}

type Endpoint struct {
	Uri          string     `json:"uri"`
	Type         string     `json:"type"`
	Method       string     `json:"method"`
	ResponseCode int        `json:"response_code"`
	Template     string     `json:"template"`
	Variables    []Variable `json:"variables"`
}

type Variable struct {
	Name   string `json:"name"`
	EnvVar string `json:"env_var"`
	Header string `json:"header"`
	Value  string `json:"value"`
}

type Logging struct {
	RequestContents  bool `json:"request_contents"`
	ResponseContents bool `json:"response_contents"`
}

type MockHandler struct {
	Config Config
	Log    *log.Logger
}

func (s MockHandler) ValidateConfig() error {
	if s.Config.ProxyPass != "" {
		if _, err := url.Parse(s.Config.ProxyPass); err != nil {
			return err
		}
	}
	for _, endpoint := range s.Config.Endpoints {
		if endpoint.Uri == "" {
			return errors.New("endpoint must include uri, e.g. /example")
		}

		if endpoint.ResponseCode == 0 {
			return errors.New("endpoint must include response_code, e.g. 200")
		}

		validMethod := false
		for _, method := range validHTTPMethods {
			if strings.ToUpper(endpoint.Method) == method {
				validMethod = true
				break
			}
		}
		if !validMethod {
			return fmt.Errorf("invalid HTTP method (%s) for endpoint %s. Allowed: %+v", endpoint.Method, endpoint.Uri, validHTTPMethods)
		}

		validType := false
		for _, eType := range validEndpointTypes {
			if strings.ToLower(endpoint.Type) == eType {
				validType = true
				break
			}
		}
		if !validType {
			return fmt.Errorf("invalid endpoint type (%s) for endpoint %s. Allowed: %+v", endpoint.Type, endpoint.Uri, validEndpointTypes)
		}

		if endpoint.Template != "" {
			rendered, err := s.RenderTemplateResponse(endpoint, &http.Request{})
			if err != nil {
				return fmt.Errorf("unable to render template %s: %+v", endpoint.Template, err)
			}

			if !IsJSON(rendered) {
				return fmt.Errorf("rendering template %s returns invalid JSON: %s", endpoint.Template, rendered)
			}
		}
	}

	return nil
}

func (s MockHandler) RenderTemplateResponse(e Endpoint, r *http.Request) (string, error) {
	template, err := os.ReadFile(e.Template)
	if err != nil {
		return "", err
	}

	t, err := fasttemplate.NewTemplate(string(template), "<", ">")
	if err != nil {
		return "", fmt.Errorf("unable to parse template %s: %+v", e.Template, err)
	}

	header := buildHeaderVars(r)
	return t.ExecuteFuncStringWithErr(func(w io.Writer, tag string) (int, error) {
		for _, variable := range e.Variables {
			if tag == variable.Name {
				if variable.EnvVar != "" {
					envVar, found := os.LookupEnv(variable.EnvVar)
					if !found {
						return 0, fmt.Errorf("env variable %s not found for template %s of endpoint %s", variable.EnvVar, e.Template, e.Uri)
					}
					return w.Write([]byte(envVar))
				}
				if variable.Value != "" {
					return w.Write([]byte(variable.Value))
				}
				if variable.Header != "" {
					if h := header.Get(variable.Header); h != "" {
						return w.Write([]byte(h))
					}
					// Headers are dynamic and only available in runtime, so fail silently if header is not set.
					s.Log.Printf("HTTP header %s not found for template %s of endpoint %s", variable.Header, e.Template, e.Uri)
					return 0, nil
				}
			}
		}

		return 0, fmt.Errorf("no matching variable found for tag %s", tag)
	})
}

func (s MockHandler) MatchEndpoint(r *http.Request) (Endpoint, error) {
	for _, endpoint := range s.Config.Endpoints {
		if strings.ToUpper(endpoint.Method) == r.Method {
			if endpoint.Type == EndpointTypeRegex {
				if match, _ := regexp.MatchString(endpoint.Uri, r.RequestURI); match {
					return endpoint, nil
				}
			}

			if endpoint.Type == EndpointTypeNormal && endpoint.Uri == r.RequestURI {
				return endpoint, nil
			}
		}
	}

	return Endpoint{}, ErrEndpointNotFound
}

func (s MockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.Config.Logging.RequestContents && r.ContentLength > 0 && r.Header.Get("Content-Type") == "application/json" {
		var rawBody bytes.Buffer
		if _, err := rawBody.ReadFrom(r.Body); err != nil {
			s.Log.Printf("Error reading request body: %s\n", err.Error())
		}
		r.Body = io.NopCloser(&rawBody) // so we can proxy it forward

		body, err := prettifyJSONBody(rawBody.Bytes())
		if err != nil {
			s.Log.Printf("Error fetching request body: %s\n", err.Error())
		} else {
			r = addToRequestContext(r, ContextKeyReqBody, body)
		}
	}

	endpoint, err := s.MatchEndpoint(r)
	if err != nil {
		if errors.Is(err, ErrEndpointNotFound) {
			if s.Config.ProxyPass != "" {
				proxyHandler(s.Config.ProxyPass, w, r, s.Log, s.Config.Logging.ResponseContents)
				return
			}

			logRequestAndRespond(s.Log, r, w, http.StatusNotFound, "Not found\n", false, false)
			return
		}

		errorHandler(w, r, s.Log, fmt.Sprintf("Unknown error in endpoint matching for %s (%s)", endpoint.Uri, endpoint.Method), err, false)
		return
	}

	if endpoint.Template == "" {
		logRequestAndRespond(s.Log, r, w, endpoint.ResponseCode, "", false, false)
		return
	}

	resp, err := s.RenderTemplateResponse(endpoint, r)
	if err != nil {
		errorHandler(w, r, s.Log, fmt.Sprintf("Unknown error in parsing response for %s (%s)", endpoint.Uri, endpoint.Method), err, false)
		return
	}

	if s.Config.Logging.ResponseContents {
		r = addToRequestContext(r, ContextKeyResBody, censorVariables(resp, endpoint.Variables))
	}

	logRequestAndRespond(s.Log, r, w, endpoint.ResponseCode, resp, true, false)
}

func proxyHandler(proxyURL string, w http.ResponseWriter, r *http.Request, l *log.Logger, logResponse bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pu, _ := url.Parse(proxyURL)
	u := r.URL
	u.Host = pu.Host
	u.Scheme = pu.Scheme
	req, err := http.NewRequestWithContext(ctx, r.Method, u.String(), r.Body)
	if err != nil {
		errorHandler(w, r, l, "Error creating a proxyable request", err, true)
		return
	}
	req.Header = r.Header.Clone()
	client := &http.Client{Timeout: time.Second * 30}
	resp, err := client.Do(req)
	if err != nil {
		errorHandler(w, r, l, "Error connecting to proxied destination", err, true)
		return
	}
	defer resp.Body.Close()
	for k, values := range resp.Header {
		for _, v := range values {
			w.Header().Set(k, v)
		}
	}

	rawResBody, err := io.ReadAll(resp.Body)
	if err != nil {
		errorHandler(w, r, l, "Error reading proxied response body", err, true)
		return
	}
	resBody := string(rawResBody)

	if logResponse {
		r = addToRequestContext(r, ContextKeyResBody, resBody)
	}

	logRequestAndRespond(l, r, w, resp.StatusCode, resBody, IsJSON(resBody), true)
}

func OpenConfigFile(path string) (Config, error) {
	configFile, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var c Config
	if err = json.Unmarshal(configFile, &c); err != nil {
		return Config{}, err
	}

	return c, nil
}

func errorHandler(w http.ResponseWriter, r *http.Request, l *log.Logger, msg string, err error, proxied bool) {
	l.Printf("%s: %v\n", msg, err)
	logRequestAndRespond(l, r, w, http.StatusInternalServerError, "Unknown error has occurred, see logs\n", false, proxied)
}

func buildHeaderVars(r *http.Request) http.Header {
	if r == nil {
		return http.Header{}
	}
	h := r.Header.Clone()
	if username, password, ok := r.BasicAuth(); ok {
		h.Set("authorization_username", username)
		h.Set("authorization_password", password)
	}
	return h
}
