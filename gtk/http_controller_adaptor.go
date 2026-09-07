package gtk

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse is the standard error response format
type ErrorResponse struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// ControllerResponse represents a successful HTTP response
// StatusCode defaults to 200 if not set or set to 0.
// Body is the response data to be JSON-encoded.
type ControllerResponse struct {
	StatusCode int               `json:"-"`
	Body       any               `json:"-"`
	Headers    map[string]string `json:"-"`
}

// ControllerFunc is the common signature for all HTTP controllers.
// Controllers return a response object and an error.
// Controllers MUST NOT write anything to the ResponseWriter.
// On error, they return an error value which will be handled centrally.
// On success, they return a ControllerResponse with the desired status code and body.
type ControllerFunc func(w http.ResponseWriter, r *http.Request) (*ControllerResponse, error)

// HttpControllerAdaptor converts a ControllerFunc into a standard http.HandlerFunc.
// Wrap c with ControllerErrorDecorator before calling this to map errors to responses.
func HttpControllerAdaptor(c ControllerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := c(w, r)
		if err != nil {
			HttpWriteErrorWithContext(w, r, err)
			return
		}
		if resp == nil {
			return
		}

		for k, v := range resp.Headers {
			w.Header().Set(k, v)
		}

		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/json")
		}

		statusCode := resp.StatusCode
		if statusCode == 0 {
			statusCode = http.StatusOK
		}
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(resp.Body)
	}
}
