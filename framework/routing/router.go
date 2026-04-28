package routing

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	onihttp "github.com/onipixel/oniworks/framework/http"
)

// Router is the OniWorks HTTP router. It matches incoming requests against
// registered routes and dispatches them through the middleware chain.
type Router struct {
	mu          sync.RWMutex
	routes      map[string][]*compiledRoute // method → routes
	allRoutes   []*Route
	namedRoutes map[string]*compiledRoute
	middleware  []onihttp.MiddlewareFunc

	// notFoundHandler is called when no route matches.
	notFoundHandler onihttp.HandlerFunc
	// errorHandler is called when a handler returns a non-nil error.
	errorHandler func(*onihttp.Context, error)
}

// New creates a new Router with sensible defaults.
func New() *Router {
	r := &Router{
		routes:      make(map[string][]*compiledRoute),
		namedRoutes: make(map[string]*compiledRoute),
	}
	r.notFoundHandler = defaultNotFound
	r.errorHandler = defaultErrorHandler
	return r
}

// Use appends global middleware that runs before every route handler.
func (r *Router) Use(mw ...onihttp.MiddlewareFunc) {
	r.mu.Lock()
	r.middleware = append(r.middleware, mw...)
	r.mu.Unlock()
}

// Group creates a route group with a shared prefix and optional middleware.
func (r *Router) Group(prefix string, fn func(*Group), mw ...onihttp.MiddlewareFunc) {
	g := &Group{
		prefix:      prefix,
		middlewares: mw,
		router:      r,
	}
	fn(g)
}

// --- Route registration shortcuts ---

func (r *Router) Get(path string, handler onihttp.HandlerFunc, mw ...onihttp.MiddlewareFunc) *Route {
	return r.addRoute("GET", path, handler, mw)
}
func (r *Router) Post(path string, handler onihttp.HandlerFunc, mw ...onihttp.MiddlewareFunc) *Route {
	return r.addRoute("POST", path, handler, mw)
}
func (r *Router) Put(path string, handler onihttp.HandlerFunc, mw ...onihttp.MiddlewareFunc) *Route {
	return r.addRoute("PUT", path, handler, mw)
}
func (r *Router) Patch(path string, handler onihttp.HandlerFunc, mw ...onihttp.MiddlewareFunc) *Route {
	return r.addRoute("PATCH", path, handler, mw)
}
func (r *Router) Delete(path string, handler onihttp.HandlerFunc, mw ...onihttp.MiddlewareFunc) *Route {
	return r.addRoute("DELETE", path, handler, mw)
}
func (r *Router) Head(path string, handler onihttp.HandlerFunc, mw ...onihttp.MiddlewareFunc) *Route {
	return r.addRoute("HEAD", path, handler, mw)
}
func (r *Router) Options(path string, handler onihttp.HandlerFunc, mw ...onihttp.MiddlewareFunc) *Route {
	return r.addRoute("OPTIONS", path, handler, mw)
}
func (r *Router) Any(path string, handler onihttp.HandlerFunc, mw ...onihttp.MiddlewareFunc) []*Route {
	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	routes := make([]*Route, 0, len(methods))
	for _, m := range methods {
		routes = append(routes, r.addRoute(m, path, handler, mw))
	}
	return routes
}

// NotFound sets a custom handler for unmatched routes.
func (r *Router) NotFound(h onihttp.HandlerFunc) { r.notFoundHandler = h }

// OnError sets a global error handler invoked when a handler returns error.
func (r *Router) OnError(h func(*onihttp.Context, error)) { r.errorHandler = h }

// addRoute registers a route internally and returns the *Route for chaining.
func (r *Router) addRoute(method, pattern string, handler onihttp.HandlerFunc, mw []onihttp.MiddlewareFunc) *Route {
	cr := &compiledRoute{
		method:     strings.ToUpper(method),
		pattern:    pattern,
		segments:   parsePattern(pattern),
		handler:    handler,
		middleware: mw,
	}
	route := &Route{
		method:      cr.method,
		path:        pattern,
		handler:     handler,
		middlewares: mw,
	}

	r.mu.Lock()
	r.routes[cr.method] = append(r.routes[cr.method], cr)
	r.allRoutes = append(r.allRoutes, route)
	r.mu.Unlock()

	// hook the compiled route back so Name() can register it
	route.compiled = cr
	route.router = r
	return route
}

// URL generates a URL for a named route, substituting params.
//
//	r.URL("user.show", "id", "42") → "/users/42"
func (r *Router) URL(name string, params ...string) (string, error) {
	r.mu.RLock()
	cr, ok := r.namedRoutes[name]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("router: no route named %q", name)
	}
	kv := make(map[string]string, len(params)/2)
	for i := 0; i+1 < len(params); i += 2 {
		kv[params[i]] = params[i+1]
	}
	var b strings.Builder
	for _, seg := range cr.segments {
		b.WriteByte('/')
		switch {
		case seg.wild:
			if v, ok := kv[seg.param]; ok {
				b.WriteString(v)
			}
		case seg.param != "":
			if v, ok := kv[seg.param]; ok {
				b.WriteString(v)
			} else {
				return "", fmt.Errorf("router: missing param %q for route %q", seg.param, name)
			}
		default:
			b.WriteString(seg.lit)
		}
	}
	return b.String(), nil
}

// Routes returns a snapshot of all registered routes (for `oni route:list`).
func (r *Router) Routes() []*Route {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Route, len(r.allRoutes))
	copy(out, r.allRoutes)
	return out
}

// ServeHTTP implements http.Handler so Router can be passed directly to http.Server.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	routes := r.routes[req.Method]
	globalMW := r.middleware
	r.mu.RUnlock()

	var (
		matchedCR *compiledRoute
		params    map[string]string
	)

	path := req.URL.Path
	if path == "" {
		path = "/"
	}

	for _, cr := range routes {
		if p, ok := cr.match(path); ok {
			matchedCR = cr
			params = p
			break
		}
	}

	c := onihttp.NewContext(w, req, params)

	if matchedCR == nil {
		if err := r.runWithMiddleware(c, globalMW, r.notFoundHandler); err != nil {
			r.errorHandler(c, err)
		}
		return
	}

	// Build middleware chain: global → route-level → handler
	allMW := make([]onihttp.MiddlewareFunc, 0, len(globalMW)+len(matchedCR.middleware))
	allMW = append(allMW, globalMW...)
	allMW = append(allMW, matchedCR.middleware...)

	if err := r.runWithMiddleware(c, allMW, matchedCR.handler); err != nil {
		r.errorHandler(c, err)
	}
}

func (r *Router) runWithMiddleware(c *onihttp.Context, mw []onihttp.MiddlewareFunc, h onihttp.HandlerFunc) error {
	final := h
	for i := len(mw) - 1; i >= 0; i-- {
		final = mw[i](final)
	}
	return final(c)
}

// registerNamed is called by Route.Name() to register the compiled route by name.
func (r *Router) registerNamed(name string, cr *compiledRoute) {
	r.mu.Lock()
	r.namedRoutes[name] = cr
	r.mu.Unlock()
}

func defaultNotFound(c *onihttp.Context) error {
	return c.JSON(http.StatusNotFound, onihttp.Map{
		"error": "not found",
		"path":  c.Path(),
	})
}

func defaultErrorHandler(c *onihttp.Context, err error) {
	var httpErr *onihttp.HTTPError
	code := http.StatusInternalServerError
	msg := "internal server error"

	// Walk the error chain to find an HTTPError
	e := err
	for e != nil {
		if he, ok := e.(*onihttp.HTTPError); ok {
			httpErr = he
			break
		}
		// unwrap
		type unwrapper interface{ Unwrap() error }
		if u, ok := e.(unwrapper); ok {
			e = u.Unwrap()
		} else {
			break
		}
	}

	if httpErr != nil {
		code = httpErr.Code
		msg = httpErr.Message
	}
	_ = c.JSON(code, onihttp.Map{"error": msg})
}
