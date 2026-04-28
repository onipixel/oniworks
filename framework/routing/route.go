package routing

import onihttp "github.com/onipixel/oniworks/framework/http"

// Route represents a single registered HTTP route.
type Route struct {
	method      string
	path        string
	handler     onihttp.HandlerFunc
	middlewares []onihttp.MiddlewareFunc
	name        string

	// back-references for post-registration configuration
	compiled *compiledRoute
	router   *Router
}

// Name sets a symbolic name for the route (used by URL generation via r.URL("name",...)).
func (r *Route) Name(name string) *Route {
	r.name = name
	if r.compiled != nil {
		r.compiled.name = name
	}
	if r.router != nil {
		r.router.registerNamed(name, r.compiled)
	}
	return r
}

// Middleware appends route-level middleware (runs after group/global middleware).
func (r *Route) Middleware(mw ...onihttp.MiddlewareFunc) *Route {
	r.middlewares = append(r.middlewares, mw...)
	if r.compiled != nil {
		r.compiled.middleware = append(r.compiled.middleware, mw...)
	}
	return r
}

// GetMethod returns the HTTP method.
func (r *Route) GetMethod() string { return r.method }

// GetPath returns the URL pattern.
func (r *Route) GetPath() string { return r.path }

// GetName returns the route name.
func (r *Route) GetName() string { return r.name }
