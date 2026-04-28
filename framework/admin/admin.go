// Package admin provides Oni Admin — an auto-generated CRUD management panel.
// Resources are registered with their model type and the panel generates
// list, create, edit, and delete routes automatically.
package admin

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/onipixel/oniworks/framework/database"
)

// Resource describes a model that the admin panel should expose.
type Resource struct {
	// Name is the human-readable resource name (e.g. "User").
	Name string
	// Slug is the URL segment (e.g. "users"). Defaults to lowercase Name + "s".
	Slug string
	// Model is a pointer to the model struct (for reflection).
	Model any
	// Columns lists which fields to show in the list view. Empty = all exported.
	Columns []string
	// Searchable is the column name to use for search (default: first string col).
	Searchable string
	// PerPage controls pagination (default: 25).
	PerPage int
}

// Panel is the Oni Admin panel.
type Panel struct {
	db        *database.DB
	resources []*Resource
	prefix    string // URL prefix, e.g. "/admin"
	title     string
	tmpl      *template.Template
}

// New creates an Admin panel bound to the given DB.
func New(db *database.DB, opts ...Option) *Panel {
	p := &Panel{
		db:     db,
		prefix: "/admin",
		title:  "Oni Admin",
	}
	for _, o := range opts {
		o(p)
	}
	p.tmpl = buildTemplates()
	return p
}

// Option configures the panel.
type Option func(*Panel)

// WithPrefix sets the URL prefix (default: "/admin").
func WithPrefix(prefix string) Option {
	return func(p *Panel) { p.prefix = strings.TrimRight(prefix, "/") }
}

// WithTitle sets the panel title.
func WithTitle(title string) Option {
	return func(p *Panel) { p.title = title }
}

// Register adds a model resource to the admin panel.
func (p *Panel) Register(r *Resource) {
	if r.Slug == "" {
		r.Slug = strings.ToLower(r.Name) + "s"
	}
	if r.PerPage <= 0 {
		r.PerPage = 25
	}
	if len(r.Columns) == 0 {
		r.Columns = exportedFields(r.Model)
	}
	p.resources = append(p.resources, r)
}

// Handler returns an http.Handler for the admin panel.
// Mount it on your router at the panel prefix.
func (p *Panel) Handler() http.Handler {
	mux := http.NewServeMux()

	// Dashboard
	mux.HandleFunc(p.prefix+"/", p.handleDashboard)

	for _, r := range p.resources {
		r := r
		slug := r.Slug

		// List
		mux.HandleFunc(p.prefix+"/"+slug, func(w http.ResponseWriter, req *http.Request) {
			p.handleList(w, req, r)
		})
		// Create form
		mux.HandleFunc(p.prefix+"/"+slug+"/new", func(w http.ResponseWriter, req *http.Request) {
			p.handleNew(w, req, r)
		})
		// Edit / Delete
		mux.HandleFunc(p.prefix+"/"+slug+"/", func(w http.ResponseWriter, req *http.Request) {
			p.handleEdit(w, req, r)
		})
	}

	return mux
}

// ─────────────────────────── Handlers ─────────────────────────────

func (p *Panel) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != p.prefix+"/" && r.URL.Path != p.prefix {
		http.NotFound(w, r)
		return
	}
	p.render(w, "dashboard.html", map[string]any{
		"Title":     p.title,
		"Resources": p.resources,
	})
}

func (p *Panel) handleList(w http.ResponseWriter, r *http.Request, res *Resource) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	search := r.URL.Query().Get("q")

	qb := p.db.Table(res.Slug)
	if search != "" && res.Searchable != "" {
		qb = qb.Where(res.Searchable+" ILIKE ?", "%"+search+"%")
	}
	qb = qb.Limit(res.PerPage).Offset((page - 1) * res.PerPage)

	var rows []map[string]any
	if err := qb.Scan(&rows); err != nil {
		http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	total, _ := p.db.Table(res.Slug).Count()

	p.render(w, "list.html", map[string]any{
		"Title":    p.title,
		"Resource": res,
		"Rows":     rows,
		"Total":    total,
		"Page":     page,
		"PerPage":  res.PerPage,
		"Search":   search,
		"Prefix":   p.prefix,
	})
}

func (p *Panel) handleNew(w http.ResponseWriter, req *http.Request, res *Resource) {
	if req.Method == http.MethodPost {
		_ = req.ParseForm()
		row := make(map[string]any)
		for _, col := range res.Columns {
			row[col] = req.FormValue(col)
		}
		if err := p.db.Table(res.Slug).Insert(row); err != nil {
			http.Error(w, "insert failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, req, p.prefix+"/"+res.Slug, http.StatusFound)
		return
	}
	p.render(w, "form.html", map[string]any{
		"Title":    p.title,
		"Resource": res,
		"Row":      nil,
		"Prefix":   p.prefix,
	})
}

func (p *Panel) handleEdit(w http.ResponseWriter, req *http.Request, res *Resource) {
	// Extract ID from URL
	parts := strings.Split(strings.TrimPrefix(req.URL.Path, p.prefix+"/"+res.Slug+"/"), "/")
	id := parts[0]
	if id == "" {
		http.NotFound(w, req)
		return
	}

	if req.Method == http.MethodPost {
		action := req.FormValue("_method")
		if action == "DELETE" {
			if err := p.db.Table(res.Slug).Where("id = ?", id).Delete(); err != nil {
				http.Error(w, "delete failed: "+err.Error(), http.StatusInternalServerError)
				return
			}
			http.Redirect(w, req, p.prefix+"/"+res.Slug, http.StatusFound)
			return
		}
		_ = req.ParseForm()
		row := make(map[string]any)
		for _, col := range res.Columns {
			row[col] = req.FormValue(col)
		}
		if err := p.db.Table(res.Slug).Where("id = ?", id).Update(database.Map(row)); err != nil {
			http.Error(w, "update failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, req, p.prefix+"/"+res.Slug, http.StatusFound)
		return
	}

	var row map[string]any
	if err := p.db.Table(res.Slug).Where("id = ?", id).Scan(&row); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	p.render(w, "form.html", map[string]any{
		"Title":    p.title,
		"Resource": res,
		"Row":      row,
		"ID":       id,
		"Prefix":   p.prefix,
	})
}

// ─────────────────────────── JSON API (optional) ──────────────────

// APIHandler returns a simple REST JSON API for all registered resources.
// Mount at a separate prefix for headless admin API access.
func (p *Panel) APIHandler() http.Handler {
	mux := http.NewServeMux()
	for _, r := range p.resources {
		r := r
		slug := r.Slug
		mux.HandleFunc("/"+slug, func(w http.ResponseWriter, req *http.Request) {
		var rows []map[string]any
		if err := p.db.Table(slug).Limit(r.PerPage).Scan(&rows); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, rows)
		})
	}
	return mux
}

// ─────────────────────────── Templates ────────────────────────────

func (p *Panel) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := p.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, fmt.Sprintf("template error: %v", err), http.StatusInternalServerError)
	}
}

func buildTemplates() *template.Template {
	t := template.New("admin").Funcs(template.FuncMap{
		"json": func(v any) string {
			b, _ := json.MarshalIndent(v, "", "  ")
			return string(b)
		},
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i + 1
			}
			return s
		},
	})

	template.Must(t.New("layout.html").Parse(layoutTmpl))
	template.Must(t.New("dashboard.html").Parse(dashboardTmpl))
	template.Must(t.New("list.html").Parse(listTmpl))
	template.Must(t.New("form.html").Parse(formTmpl))
	return t
}

// ─────────────────────────── Helpers ──────────────────────────────

func exportedFields(model any) []string {
	t := reflect.TypeOf(model)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	var fields []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.IsExported() {
			tag := f.Tag.Get("db")
			if tag == "-" {
				continue
			}
			name := tag
			if name == "" {
				name = strings.ToLower(f.Name)
			}
			fields = append(fields, name)
		}
	}
	return fields
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ─────────────────────────── HTML templates ───────────────────────

const layoutTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:system-ui,sans-serif;background:#f5f5f5;color:#333}
nav{background:#1a1a2e;color:#fff;padding:1rem 2rem;display:flex;gap:2rem;align-items:center}
nav a{color:#a0c4ff;text-decoration:none;font-weight:500}
nav a:hover{color:#fff}
.container{max-width:1200px;margin:2rem auto;padding:0 2rem}
table{width:100%;border-collapse:collapse;background:#fff;border-radius:8px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,.1)}
th,td{padding:.75rem 1rem;text-align:left;border-bottom:1px solid #e5e7eb}
th{background:#f9fafb;font-weight:600;font-size:.875rem;text-transform:uppercase;letter-spacing:.05em;color:#6b7280}
tr:hover td{background:#f9fafb}
.btn{display:inline-block;padding:.5rem 1rem;border-radius:6px;font-size:.875rem;font-weight:500;cursor:pointer;text-decoration:none;border:none}
.btn-primary{background:#4f46e5;color:#fff}.btn-primary:hover{background:#4338ca}
.btn-danger{background:#ef4444;color:#fff}.btn-danger:hover{background:#dc2626}
.btn-sm{padding:.25rem .75rem;font-size:.8rem}
.card{background:#fff;border-radius:8px;padding:1.5rem;box-shadow:0 1px 3px rgba(0,0,0,.1)}
input,select{padding:.5rem .75rem;border:1px solid #d1d5db;border-radius:6px;font-size:.9rem;width:100%}
.form-group{margin-bottom:1rem}
label{display:block;font-size:.875rem;font-weight:500;margin-bottom:.25rem;color:#374151}
.pagination{display:flex;gap:.5rem;margin-top:1rem}
.page-btn{padding:.375rem .75rem;border:1px solid #d1d5db;border-radius:4px;text-decoration:none;color:#374151;font-size:.875rem}
.page-btn.active{background:#4f46e5;color:#fff;border-color:#4f46e5}
</style>
</head>
<body>
<nav>
<a href="/admin">{{.Title}}</a>
{{range .Resources}}<a href="/admin/{{.Slug}}">{{.Name}}</a>{{end}}
</nav>
<div class="container">
{{block "content" .}}{{end}}
</div>
</body>
</html>`

const dashboardTmpl = `{{template "layout.html" .}}
{{define "content"}}
<h2 style="margin-bottom:1.5rem">Dashboard</h2>
<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:1rem">
{{range .Resources}}
<a href="/admin/{{.Slug}}" style="text-decoration:none">
<div class="card" style="text-align:center;cursor:pointer">
<h3 style="color:#4f46e5;font-size:1.5rem">{{.Name}}</h3>
<p style="color:#6b7280;margin-top:.5rem">Manage</p>
</div></a>
{{end}}
</div>
{{end}}`

const listTmpl = `{{template "layout.html" .}}
{{define "content"}}
<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:1rem">
<h2>{{.Resource.Name}}</h2>
<a href="{{.Prefix}}/{{.Resource.Slug}}/new" class="btn btn-primary">+ New {{.Resource.Name}}</a>
</div>
<form method="GET" style="margin-bottom:1rem">
<input name="q" value="{{.Search}}" placeholder="Search..." style="width:300px">
</form>
<table>
<thead><tr>{{range .Resource.Columns}}<th>{{.}}</th>{{end}}<th>Actions</th></tr></thead>
<tbody>
{{range .Rows}}
<tr>
{{range $.Resource.Columns}}<td>{{index $ .}}</td>{{end}}
<td><a href="{{$.Prefix}}/{{$.Resource.Slug}}/{{index . "id"}}" class="btn btn-sm btn-primary">Edit</a></td>
</tr>
{{else}}<tr><td colspan="99" style="text-align:center;color:#9ca3af">No records found.</td></tr>
{{end}}
</tbody>
</table>
{{end}}`

const formTmpl = `{{template "layout.html" .}}
{{define "content"}}
<div style="max-width:600px">
<h2 style="margin-bottom:1.5rem">{{if .Row}}Edit{{else}}New{{end}} {{.Resource.Name}}</h2>
<div class="card">
<form method="POST">
{{if .Row}}<input type="hidden" name="_method" value="PUT">{{end}}
{{range .Resource.Columns}}
<div class="form-group">
<label>{{.}}</label>
<input name="{{.}}" value="{{if $.Row}}{{index $.Row .}}{{end}}">
</div>
{{end}}
<div style="display:flex;gap:1rem;margin-top:1rem">
<button type="submit" class="btn btn-primary">Save</button>
<a href="{{.Prefix}}/{{.Resource.Slug}}" class="btn" style="background:#e5e7eb">Cancel</a>
{{if .Row}}
<form method="POST" style="display:inline" onsubmit="return confirm('Delete?')">
<input type="hidden" name="_method" value="DELETE">
<button class="btn btn-danger btn-sm">Delete</button>
</form>
{{end}}
</div>
</form>
</div>
</div>
{{end}}`
