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
// It calls the controller and handles the response:
// - On error, it panics with the error (to be handled by HttpErrorMiddleware)
// - On success, it writes the response to ResponseWriter with proper status code
func HttpControllerAdaptor(c ControllerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := c(w, r)
		if err != nil {
			panic(err)
		}

		// Set headers
		for k, v := range resp.Headers {
			w.Header().Set(k, v)
		}

		// Set content type if not already set
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/json")
		}

		// Set status code (default to 200 if not specified or 0)
		statusCode := resp.StatusCode
		if statusCode == 0 {
			statusCode = http.StatusOK
		}
		w.WriteHeader(statusCode)

		// Encode response body
		json.NewEncoder(w).Encode(resp.Body)
	}
}
