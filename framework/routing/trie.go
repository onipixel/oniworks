// Package routing provides a high-performance segment-based HTTP router.
package routing

import (
	"strings"

	onihttp "github.com/onipixel/oniworks/framework/http"
)

// segment represents one path component after splitting on "/".
type segment struct {
	lit   string // literal text (if not param/wild)
	param string // parameter name (non-empty for :name segments)
	wild  bool   // true for *rest wildcard (matches everything remaining)
}

// compiledRoute is a route with its path parsed into matchable segments.
type compiledRoute struct {
	method     string
	pattern    string
	segments   []segment
	handler    onihttp.HandlerFunc
	middleware []onihttp.MiddlewareFunc
	name       string
}

// parsePattern splits a URL pattern into typed segments.
// Both ":name" and "{name}" are accepted as named parameters.
// "/users/:id/posts/*rest" → [{lit:"users"}, {param:"id"}, {lit:"posts"}, {wild:true,param:"rest"}]
func parsePattern(pattern string) []segment {
	parts := splitPath(pattern)
	segs := make([]segment, 0, len(parts))
	for _, p := range parts {
		switch {
		case p == "":
			continue
		case strings.HasPrefix(p, "*"):
			name := p[1:]
			if name == "" {
				name = "*"
			}
			segs = append(segs, segment{wild: true, param: name})
			break // wildcard eats the rest
		case strings.HasPrefix(p, ":"):
			segs = append(segs, segment{param: p[1:]})
		case strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}"):
			segs = append(segs, segment{param: p[1 : len(p)-1]})
		default:
			segs = append(segs, segment{lit: p})
		}
	}
	return segs
}

// match attempts to match path against cr, filling params on success.
func (cr *compiledRoute) match(path string) (map[string]string, bool) {
	parts := splitPath(path)
	segs := cr.segments
	params := map[string]string{}
	pi := 0

	for _, seg := range segs {
		switch {
		case seg.wild:
			// wildcard eats all remaining path parts
			params[seg.param] = strings.Join(parts[pi:], "/")
			return params, true
		case seg.param != "":
			if pi >= len(parts) {
				return nil, false
			}
			params[seg.param] = parts[pi]
			pi++
		default:
			if pi >= len(parts) || parts[pi] != seg.lit {
				return nil, false
			}
			pi++
		}
	}

	// All segments consumed AND path fully consumed
	if pi != len(parts) {
		return nil, false
	}
	return params, true
}

// splitPath splits a URL path by "/" ignoring leading/trailing slashes.
func splitPath(p string) []string {
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}
