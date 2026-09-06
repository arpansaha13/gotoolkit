package gtk

import (
	"log"
	"net/http"
)

// HttpRecoveryMiddleware is a minimal recovery middleware that catches any
// uncaught panics and returns a generic 500 error response.
// It should be the outermost middleware in the chain.
func HttpRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				// Log the panic for debugging
				log.Printf("panic recovered by HttpRecoveryMiddleware: %v, type: %T", rec, rec)

				// Return generic error response
				writeErrorResponse(w, r, http.StatusInternalServerError, "Something went wrong!", "INTERNAL_ERROR")
			}
		}()

		next.ServeHTTP(w, r)
	})
}
