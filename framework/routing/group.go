package routing

import onihttp "github.com/oniworks/oniworks/framework/http"

// Group is a route group with a shared prefix and middleware stack.
type Group struct {
	prefix      string
	middlewares []onihttp.MiddlewareFunc
	router      *Router
}

// Use appends middleware to this group. Returns the group for chaining.
func (g *Group) Use(mw ...onihttp.MiddlewareFunc) *Group {
	g.middlewares = append(g.middlewares, mw...)
	return g
}

// Group creates a nested sub-group under this group's prefix.
func (g *Group) Group(prefix string, fn func(*Group)) {
	sub := &Group{
		prefix:      g.prefix + prefix,
		middlewares: append([]onihttp.MiddlewareFunc{}, g.middlewares...),
		router:      g.router,
	}
	fn(sub)
}

// Get registers a GET route on this group.
func (g *Group) Get(path string, handler onihttp.HandlerFunc, mw ...onihttp.MiddlewareFunc) *Route {
	return g.handle("GET", path, handler, mw...)
}

// Post registers a POST route on this group.
func (g *Group) Post(path string, handler onihttp.HandlerFunc, mw ...onihttp.MiddlewareFunc) *Route {
	return g.handle("POST", path, handler, mw...)
}

// Put registers a PUT route on this group.
func (g *Group) Put(path string, handler onihttp.HandlerFunc, mw ...onihttp.MiddlewareFunc) *Route {
	return g.handle("PUT", path, handler, mw...)
}

// Patch registers a PATCH route on this group.
func (g *Group) Patch(path string, handler onihttp.HandlerFunc, mw ...onihttp.MiddlewareFunc) *Route {
	return g.handle("PATCH", path, handler, mw...)
}

// Delete registers a DELETE route on this group.
func (g *Group) Delete(path string, handler onihttp.HandlerFunc, mw ...onihttp.MiddlewareFunc) *Route {
	return g.handle("DELETE", path, handler, mw...)
}

// Head registers a HEAD route on this group.
func (g *Group) Head(path string, handler onihttp.HandlerFunc, mw ...onihttp.MiddlewareFunc) *Route {
	return g.handle("HEAD", path, handler, mw...)
}

// Options registers an OPTIONS route on this group.
func (g *Group) Options(path string, handler onihttp.HandlerFunc, mw ...onihttp.MiddlewareFunc) *Route {
	return g.handle("OPTIONS", path, handler, mw...)
}

// Any registers a route for all common HTTP methods.
func (g *Group) Any(path string, handler onihttp.HandlerFunc, mw ...onihttp.MiddlewareFunc) []*Route {
	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	routes := make([]*Route, 0, len(methods))
	for _, m := range methods {
		routes = append(routes, g.handle(m, path, handler, mw...))
	}
	return routes
}

func (g *Group) handle(method, path string, handler onihttp.HandlerFunc, extra ...onihttp.MiddlewareFunc) *Route {
	fullPath := g.prefix + path
	// Merge: global router mw → group mw → route-level mw
	mw := make([]onihttp.MiddlewareFunc, 0, len(g.middlewares)+len(extra))
	mw = append(mw, g.middlewares...)
	mw = append(mw, extra...)
	return g.router.addRoute(method, fullPath, handler, mw)
}
