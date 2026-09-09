package httpx

import "net/http"

// HttpRouter is a ServeMux wrapper with global and per-route middleware.
type HttpRouter struct {
	mux               *http.ServeMux
	globalMiddlewares []func(http.Handler) http.Handler
	routeMiddlewares  []func(http.Handler) http.Handler
	isRoot            bool
}

// NewHttpRouter creates a root router. globalMiddlewares wrap the mux in Handler.
func NewHttpRouter(globalMiddlewares ...func(http.Handler) http.Handler) *HttpRouter {
	return &HttpRouter{
		mux:               http.NewServeMux(),
		globalMiddlewares: globalMiddlewares,
		isRoot:            true,
	}
}

// Child returns a non-root router that shares the mux. Parent route
// middlewares are prepended to routeMiddlewares.
func (rt *HttpRouter) Child(routeMiddlewares ...func(http.Handler) http.Handler) *HttpRouter {
	combined := make([]func(http.Handler) http.Handler, 0, len(rt.routeMiddlewares)+len(routeMiddlewares))
	combined = append(combined, rt.routeMiddlewares...)
	combined = append(combined, routeMiddlewares...)
	return &HttpRouter{
		mux:               rt.mux,
		globalMiddlewares: rt.globalMiddlewares,
		routeMiddlewares:  combined,
		isRoot:            false,
	}
}

// Controller registers a ControllerFunc wrapped with ControllerErrorDecorator.
func (rt *HttpRouter) Controller(pattern string, c ControllerFunc) {
	rt.HandleFunc(pattern, HttpControllerAdaptor(ControllerErrorDecorator(c)))
}

// HandleFunc registers handler on the mux, wrapped with routeMiddlewares.
func (rt *HttpRouter) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	wrapped := chain(http.HandlerFunc(handler), rt.routeMiddlewares...)
	rt.mux.HandleFunc(pattern, wrapped.ServeHTTP)
}

// Handler returns the mux wrapped with globalMiddlewares.
// Panics if called on a child router.
func (rt *HttpRouter) Handler() http.Handler {
	if !rt.isRoot {
		panic("gtk: Handler called on child HttpRouter")
	}
	return chain(rt.mux, rt.globalMiddlewares...)
}
