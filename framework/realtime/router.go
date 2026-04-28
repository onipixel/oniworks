package realtime

import "strings"

// HandlerFunc handles an incoming realtime event.
type HandlerFunc func(c *Conn, e *Event) error

// channelRoute matches channel patterns like "chat.{room}" or "user.{id}.status".
type channelRoute struct {
	pattern  string
	segments []routeSegment
	handler  HandlerFunc
}

type routeSegment struct {
	lit   string // literal segment text
	param string // non-empty if this is a {param} segment
}

// match reports whether channel matches this route's pattern.
// On match, it fills params with captured wildcard values.
func (r *channelRoute) match(channel string) (map[string]string, bool) {
	patSegs := r.segments
	chSegs := strings.Split(channel, ".")

	if len(patSegs) != len(chSegs) {
		return nil, false
	}

	params := make(map[string]string, 2)
	for i, ps := range patSegs {
		if ps.param != "" {
			params[ps.param] = chSegs[i]
		} else if ps.lit != chSegs[i] {
			return nil, false
		}
	}
	return params, true
}

// parseChannelPattern parses "chat.{room}" into route segments.
func parseChannelPattern(pattern string) []routeSegment {
	parts := strings.Split(pattern, ".")
	segs := make([]routeSegment, len(parts))
	for i, p := range parts {
		if len(p) > 2 && p[0] == '{' && p[len(p)-1] == '}' {
			segs[i] = routeSegment{param: p[1 : len(p)-1]}
		} else {
			segs[i] = routeSegment{lit: p}
		}
	}
	return segs
}

// eventRouter matches incoming events to registered channel handlers.
type eventRouter struct {
	routes []*channelRoute
}

func newEventRouter() *eventRouter { return &eventRouter{} }

// Register adds a handler for a channel pattern.
func (er *eventRouter) Register(pattern string, fn HandlerFunc) {
	er.routes = append(er.routes, &channelRoute{
		pattern:  pattern,
		segments: parseChannelPattern(pattern),
		handler:  fn,
	})
}

// Match finds the first route matching channel and returns the handler and extracted params.
func (er *eventRouter) Match(channel string) (HandlerFunc, map[string]string) {
	for _, r := range er.routes {
		if params, ok := r.match(channel); ok {
			return r.handler, params
		}
	}
	return nil, nil
}
