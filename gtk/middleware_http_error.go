package gtk

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/arpansaha13/gotoolkit/internal/logger"
	"go.uber.org/zap"
)

// HttpErrorMiddleware recovers from panics thrown by handlers and converts
// domain errors to HTTP responses. It should be placed after HttpRecoveryMiddleware
// and LoggingMiddleware in the middleware chain.
func HttpErrorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				// Check if the panic value is an error
				err, ok := rec.(error)
				if !ok {
					// Non-error panic - treat as internal server error
					lgr := logger.LoggerFromContext(r.Context())
					lgr.Error("panic recovered (non-error)",
						zap.Any("panic_value", rec),
					)
					writeErrorResponse(w, r, http.StatusInternalServerError, "Something went wrong!", "INTERNAL_ERROR")
					return
				}

				// Unwrap the error to get the actual underlying error
				unwrappedErr := errors.Unwrap(err)
				if unwrappedErr == nil {
					unwrappedErr = err
				}

				// Log the full error details for debugging
				lgr := logger.LoggerFromContext(r.Context())
				lgr.Error("error recovered",
					zap.Error(err),
					zap.Error(unwrappedErr),
				)

				// Map error to HTTP response
				statusCode, message, code := errorToHTTP(unwrappedErr)
				writeErrorResponse(w, r, statusCode, message, code)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// HttpWriteErrorWithContext writes an error response with logging to the client
func HttpWriteErrorWithContext(w http.ResponseWriter, r *http.Request, err error) {
	lgr := logger.LoggerFromContext(r.Context())
	statusCode, message, code := errorToHTTP(err)
	lgr.Info("error response", zap.String("code", code), zap.Int("status", statusCode), zap.Error(err))
	writeErrorResponse(w, r, statusCode, message, code)
}

// writeErrorResponse writes an error response to the client.
// It sets the Content-Type and status code
// before writing the body. Headers must be set before WriteHeader is called.
func writeErrorResponse(w http.ResponseWriter, r *http.Request, statusCode int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{
		Message: message,
		Code:    code,
	})
}

// errorToHTTP translates domain errors to HTTP status codes.
// For 500 errors, it returns a generic message while logging the actual error.
func errorToHTTP(err error) (int, string, string) {
	if err == nil {
		return http.StatusOK, "", ""
	}

	// Unwrap InternalError to get the underlying error for logging
	var internalErr *InternalError
	if errors.As(err, &internalErr) && internalErr.Err != nil {
		log.Printf("internal error: %v, underlying: %v", err, internalErr.Err)
	}

	if IsValidation(err) {
		return http.StatusBadRequest, err.Error(), "VALIDATION_ERROR"
	}

	if IsConflict(err) {
		return http.StatusConflict, err.Error(), "CONFLICT_ERROR"
	}

	if IsNotFound(err) {
		return http.StatusNotFound, err.Error(), "NOT_FOUND_ERROR"
	}

	if IsUnauthorized(err) {
		return http.StatusUnauthorized, err.Error(), "UNAUTHORIZED_ERROR"
	}

	if IsForbidden(err) {
		return http.StatusForbidden, err.Error(), "FORBIDDEN_ERROR"
	}

	if IsServiceUnavailable(err) {
		return http.StatusServiceUnavailable, err.Error(), "SERVICE_UNAVAILABLE_ERROR"
	}

	// Default to internal error with generic message
	// The actual error details are logged above
	return http.StatusInternalServerError, "Something went wrong!", "INTERNAL_ERROR"
}
