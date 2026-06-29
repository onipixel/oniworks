// Package errors provides error handling utilities for OniWorks applications.
// In debug mode it renders a rich HTML error page (similar to Laravel's Ignition)
// with full stack trace, request details, and highlighted application frames.
// In production it returns a minimal JSON response.
package errors

import (
	"fmt"
	"html"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"

	onihttp "github.com/onipixel/oniworks/framework/http"
)

// HandlerForEnv returns an error handler with debug mode enabled ONLY when env
// names a development environment ("local", "dev", "development", "debug",
// case-insensitive). Any other value — including the empty string — yields a
// production handler that never leaks internals. Prefer this over Handler(true)
// so a stray config value can't turn on stack-trace disclosure in production.
func HandlerForEnv(env string) func(*onihttp.Context, error) {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "local", "dev", "development", "debug":
		return Handler(true)
	default:
		return Handler(false)
	}
}

// Handler returns a router-compatible error handler.
// When debugMode is true and the error is not an HTTPError, it renders a
// detailed HTML page for browser requests or a verbose JSON payload for API
// requests. For HTTPError values (4xx / known 5xx) it always returns clean JSON.
//
// Enabling debug mode exposes stack traces, file paths, and raw error strings;
// never enable it in production. Consider HandlerForEnv to wire this safely.
func Handler(debugMode bool) func(*onihttp.Context, error) {
	return func(c *onihttp.Context, err error) {
		// Walk the chain looking for an HTTPError.
		var httpErr *onihttp.HTTPError
		e := err
		for e != nil {
			if he, ok := e.(*onihttp.HTTPError); ok {
				httpErr = he
				break
			}
			type unwrapper interface{ Unwrap() error }
			if u, ok := e.(unwrapper); ok {
				e = u.Unwrap()
			} else {
				break
			}
		}

		if httpErr != nil {
			// Known HTTP errors → always clean JSON (no stack trace needed).
			_ = c.JSON(httpErr.Code, onihttp.Map{"error": httpErr.Message})
			return
		}

		if !debugMode {
			// Production: hide internals.
			_ = c.JSON(http.StatusInternalServerError, onihttp.Map{"error": "internal server error"})
			return
		}

		// ── Dev mode: rich error output ─────────────────────────────────────
		stack := string(debug.Stack())
		frames := parseStack(stack)

		accept := c.Request.Header.Get("Accept")
		if strings.Contains(accept, "text/html") {
			renderHTML(c, err, frames)
		} else {
			renderJSON(c, err, frames)
		}
	}
}

// ─────────────────────────── JSON dev response ───────────────────────────────

func renderJSON(c *onihttp.Context, err error, frames []stackFrame) {
	type jsonFrame struct {
		Func string `json:"func"`
		File string `json:"file"`
		Line int    `json:"line"`
		App  bool   `json:"app"`
	}
	jFrames := make([]jsonFrame, len(frames))
	for i, f := range frames {
		jFrames[i] = jsonFrame{Func: f.Func, File: f.File, Line: f.Line, App: f.IsApp}
	}
	_ = c.JSON(http.StatusInternalServerError, onihttp.Map{
		"error":   err.Error(),
		"type":    fmt.Sprintf("%T", err),
		"stack":   jFrames,
		"request": onihttp.Map{"method": c.Method(), "path": c.Path()},
	})
}

// ─────────────────────────── HTML dev page ───────────────────────────────────

func renderHTML(c *onihttp.Context, err error, frames []stackFrame) {
	errType := fmt.Sprintf("%T", err)
	msg := html.EscapeString(err.Error())

	var frameBuf strings.Builder
	for _, f := range frames {
		cls := "fw"
		if f.IsApp {
			cls = "app"
		}
		frameBuf.WriteString(fmt.Sprintf(
			`<div class="frame %s"><div class="fn">%s</div><div class="file">%s:<span class="lineno">%d</span></div></div>`,
			cls,
			html.EscapeString(f.Func),
			html.EscapeString(f.File),
			f.Line,
		))
	}

	page := strings.NewReplacer(
		"{{METHOD}}", html.EscapeString(c.Method()),
		"{{PATH}}", html.EscapeString(c.Path()),
		"{{TYPE}}", html.EscapeString(errType),
		"{{MSG}}", msg,
		"{{FRAMES}}", frameBuf.String(),
	).Replace(errorPageTemplate)

	c.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Response.WriteHeader(http.StatusInternalServerError)
	_, _ = c.Response.Write([]byte(page))
}

// ─────────────────────────── Stack parser ────────────────────────────────────

type stackFrame struct {
	Func  string
	File  string
	Line  int
	IsApp bool // true when not a stdlib / framework internal frame
}

func parseStack(raw string) []stackFrame {
	lines := strings.Split(raw, "\n")
	var frames []stackFrame

	for i := 0; i+1 < len(lines); i++ {
		fn := strings.TrimSpace(lines[i])
		fileLine := strings.TrimSpace(lines[i+1])

		// Stack lines come in pairs: function name, then "\t<file>:<line> +0x…"
		if !strings.HasPrefix(fileLine, "/") && !strings.Contains(fileLine, ":\\") {
			continue
		}

		// Strip "+0x…" suffix
		if idx := strings.LastIndex(fileLine, " +0x"); idx != -1 {
			fileLine = fileLine[:idx]
		}

		// Split file:line
		file := fileLine
		lineNum := 0
		if idx := strings.LastIndex(fileLine, ":"); idx != -1 {
			if n, err := strconv.Atoi(fileLine[idx+1:]); err == nil {
				lineNum = n
				file = fileLine[:idx]
			}
		}

		isApp := !isInternalFrame(fn, file)
		frames = append(frames, stackFrame{
			Func:  fn,
			File:  file,
			Line:  lineNum,
			IsApp: isApp,
		})
		i++ // skip the file line we just consumed
	}
	return frames
}

func isInternalFrame(fn, file string) bool {
	internals := []string{
		"runtime/", "runtime.", "testing.",
		"net/http.", "github.com/onipixel/oniworks/framework/",
		"github.com/jackc/", "github.com/golang-jwt/",
	}
	for _, p := range internals {
		if strings.Contains(fn, p) || strings.Contains(file, p) {
			return true
		}
	}
	return false
}

// ─────────────────────────── HTML template ───────────────────────────────────

const errorPageTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>500 — OniWorks Error</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:'Segoe UI',system-ui,sans-serif;background:#0d1117;color:#c9d1d9;min-height:100vh}
.banner{background:#161b22;border-bottom:3px solid #f85149;padding:28px 40px}
.badge{display:inline-block;background:#f85149;color:#fff;font-size:11px;font-weight:700;padding:3px 10px;border-radius:4px;letter-spacing:.08em;margin-bottom:14px;text-transform:uppercase}
.err-type{font-size:13px;color:#8b949e;font-family:monospace;margin-bottom:8px}
.err-msg{font-size:22px;font-weight:600;color:#f85149;line-height:1.4;margin-bottom:12px}
.req{font-size:13px;color:#8b949e}.req .method{color:#58a6ff;font-weight:600}.req .path{color:#c9d1d9}
.body{padding:32px 40px;max-width:1100px}
.card{background:#161b22;border:1px solid #30363d;border-radius:8px;margin-bottom:24px;overflow:hidden}
.card-head{padding:12px 20px;background:#1c2128;border-bottom:1px solid #30363d;font-size:11px;font-weight:700;color:#8b949e;text-transform:uppercase;letter-spacing:.1em}
.frame{padding:10px 20px;border-bottom:1px solid #21262d;font-size:13px}
.frame:last-child{border-bottom:none}
.frame.app{background:#1a2233}
.frame.fw{opacity:.45}
.fn{color:#d2a8ff;font-family:'Cascadia Code','Fira Code',Consolas,monospace;margin-bottom:3px;word-break:break-all}
.file{color:#8b949e;font-family:monospace;font-size:12px}
.lineno{color:#e3b341;font-weight:700}
.no-frames{padding:20px;color:#8b949e;font-size:13px}
.legend{padding:14px 20px;border-top:1px solid #21262d;font-size:12px;color:#8b949e;display:flex;gap:20px}
.dot{display:inline-block;width:10px;height:10px;border-radius:50%;margin-right:5px}
.dot.app{background:#388bfd}.dot.fw{background:#444c56}
</style>
</head>
<body>
<div class="banner">
  <div class="badge">500 Internal Server Error</div>
  <div class="err-type">{{TYPE}}</div>
  <div class="err-msg">{{MSG}}</div>
  <div class="req"><span class="method">{{METHOD}}</span> <span class="path">{{PATH}}</span></div>
</div>
<div class="body">
  <div class="card">
    <div class="card-head">Stack Trace</div>
    {{FRAMES}}
    <div class="legend">
      <span><span class="dot app"></span>Application frame</span>
      <span><span class="dot fw"></span>Framework / stdlib frame</span>
    </div>
  </div>
</div>
</body>
</html>`
