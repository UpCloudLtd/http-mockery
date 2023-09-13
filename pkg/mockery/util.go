package mockery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

func IsJSON(str string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(str), &js) == nil
}

func prettifyJSONBody(body []byte) (string, error) {
	var prettierJSON bytes.Buffer
	if err := json.Indent(&prettierJSON, body, "", "  "); err != nil {
		return "", fmt.Errorf("invalid JSON in body: %w", err)
	}

	return prettierJSON.String(), nil
}

func censorVariables(text string, vars []Variable) string {
	for _, variable := range vars {
		toReplace := ""

		if variable.Value != "" {
			toReplace = variable.Value
		}
		if variable.EnvVar != "" {
			envValue, found := os.LookupEnv(variable.EnvVar)
			if found {
				toReplace = envValue
			}
		}

		if toReplace != "" {
			text = strings.ReplaceAll(text, toReplace, strings.Repeat("*", len(toReplace)))
		}
	}

	return text
}

func logRequestAndRespond(l *log.Logger, r *http.Request, w http.ResponseWriter, status int, res string, jsonResponse, proxied bool) {
	logLine := "%s - \"%s %s %s\" %d \"%s\""
	if proxied {
		logLine = "Proxied for %s - \"%s %s %s\" %d \"%s\""
	}
	l.Printf(logLine, strings.Split(r.RemoteAddr, ":")[0], r.Method, r.RequestURI, r.Proto, status, r.UserAgent())
	ctx := r.Context()
	if ctx.Value(ContextKeyReqBody) != nil {
		l.Printf("Request body for %s %s:\n%s\n", r.Method, r.RequestURI, ctx.Value(ContextKeyReqBody))
	}
	if ctx.Value(ContextKeyResBody) != nil {
		l.Printf("Response body for %s %s:\n%s\n", r.Method, r.RequestURI, ctx.Value(ContextKeyResBody))
	}

	if jsonResponse && !IsJSON(res) {
		l.Printf("Response for %s %s is not expected valid JSON: %s\n", r.Method, r.RequestURI, res)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Unknown error has occurred, see logs\n"))
		return
	}

	if jsonResponse {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(status)
	_, _ = w.Write([]byte(res))
}

func addToRequestContext(r *http.Request, key, content string) *http.Request {
	ctx := r.Context()

	ctx = context.WithValue(ctx, key, content)

	return r.WithContext(ctx)
}
