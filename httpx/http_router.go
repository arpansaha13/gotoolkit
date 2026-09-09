package httpx

import "net/http"

// Router is a ServeMux wrapper with global and per-route middleware.
type Router struct {
	mux               *http.ServeMux
	globalMiddlewares []func(http.Handler) http.Handler
	routeMiddlewares  []func(http.Handler) http.Handler
	isRoot            bool
}

// NewRouter creates a root router. globalMiddlewares wrap the mux in Handler.
func NewRouter(globalMiddlewares ...func(http.Handler) http.Handler) *Router {
	return &Router{
		mux:               http.NewServeMux(),
		globalMiddlewares: globalMiddlewares,
		isRoot:            true,
	}
}

// Child returns a non-root router that shares the mux. Parent route
// middlewares are prepended to routeMiddlewares.
func (rt *Router) Child(routeMiddlewares ...func(http.Handler) http.Handler) *Router {
	combined := make([]func(http.Handler) http.Handler, 0, len(rt.routeMiddlewares)+len(routeMiddlewares))
	combined = append(combined, rt.routeMiddlewares...)
	combined = append(combined, routeMiddlewares...)
	return &Router{
		mux:               rt.mux,
		globalMiddlewares: rt.globalMiddlewares,
		routeMiddlewares:  combined,
		isRoot:            false,
	}
}

// Controller registers a ControllerFunc wrapped with ControllerErrorDecorator.
func (rt *Router) Controller(pattern string, c ControllerFunc) {
	rt.HandleFunc(pattern, ControllerAdaptor(ControllerErrorDecorator(c)))
}

// HandleFunc registers handler on the mux, wrapped with routeMiddlewares.
func (rt *Router) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	wrapped := chain(http.HandlerFunc(handler), rt.routeMiddlewares...)
	rt.mux.HandleFunc(pattern, wrapped.ServeHTTP)
}

// Handler returns the mux wrapped with globalMiddlewares.
// Panics if called on a child router.
func (rt *Router) Handler() http.Handler {
	if !rt.isRoot {
		panic("gtk: Handler called on child Router")
	}
	return chain(rt.mux, rt.globalMiddlewares...)
}
