package httpx

import (
	"net/http"
	"slices"
)

// chain wraps h with mws. First middleware is outermost.
func chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for _, mw := range slices.Backward(mws) {
		h = mw(h)
	}
	return h
}
