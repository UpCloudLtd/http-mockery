package mockery

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/valyala/fasttemplate"
)

var (
	ErrEndpointNotFound = fmt.Errorf("no matching endpoint found")
)

var validHTTPMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPost, http.MethodDelete}

const (
	EndpointTypeNormal = "normal"
	EndpointTypeRegex  = "regex"
)

var validEndpointTypes = []string{EndpointTypeNormal, EndpointTypeRegex}

type Config struct {
	ListenIP   string     `json:"listen_ip"`
	ListenPort int        `json:"listen_port"`
	Endpoints  []Endpoint `json:"endpoints"`
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
	Value  string `json:"value"`
}

type MockHandler struct {
	Config Config
}

func (s MockHandler) ValidateConfig() error {
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
			return fmt.Errorf(fmt.Sprintf("Invalid HTTP method (%s) for endpoint %s. Allowed: %+v", endpoint.Method, endpoint.Uri, validHTTPMethods))
		}

		validType := false
		for _, eType := range validEndpointTypes {
			if strings.ToLower(endpoint.Type) == eType {
				validType = true
				break
			}
		}
		if !validType {
			return fmt.Errorf(fmt.Sprintf("Invalid endpoint type (%s) for endpoint %s. Allowed: %+v", endpoint.Type, endpoint.Uri, validEndpointTypes))
		}

		if endpoint.Template != "" {
			rendered, err := s.RenderTemplateResponse(endpoint)
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

func (s MockHandler) RenderTemplateResponse(e Endpoint) (string, error) {
	template, err := os.ReadFile(e.Template)
	if err != nil {
		return "", err
	}

	t, err := fasttemplate.NewTemplate(string(template), "<", ">")
	if err != nil {
		return "", fmt.Errorf("unable to parse template %s: %+v", e.Template, err)
	}

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
	// TODO add request logging
	endpoint, err := s.MatchEndpoint(r)
	if err != nil {
		if errors.Is(err, ErrEndpointNotFound) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("Not found\n"))
			return
		}

		log.Printf("Unknown error in endpoint matching for %s (%s): %+v\n", endpoint.Uri, endpoint.Method, err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Unknown error has occurred, see logs\n"))
		return
	}

	if endpoint.Template == "" {
		w.WriteHeader(endpoint.ResponseCode)
		return
	}

	resp, err := s.RenderTemplateResponse(endpoint)
	if err != nil {
		log.Printf("Unknown error in parsing response for %s (%s): %+v\n", endpoint.Uri, endpoint.Method, err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Unknown error has occurred, see logs\n"))
		return
	}

	if !IsJSON(resp) {
		log.Printf("Response for %s (%s) is not valid JSON: %s\n", endpoint.Uri, endpoint.Method, resp)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Unknown error has occurred, see logs\n"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(endpoint.ResponseCode)
	_, _ = w.Write([]byte(resp))
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

func IsJSON(str string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(str), &js) == nil
}
