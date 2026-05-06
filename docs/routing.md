# Routing

OniWorks has a fast, trie-based HTTP router that supports named parameters, groups, and middleware chains.

## Defining Routes

Routes are registered inside `oni.Route(func(r *routing.Router) { ... })`.

```go
oni.Route(func(r *routing.Router) {
    r.Get("/", HomeHandler)
    r.Post("/contact", ContactHandler)
    r.Put("/users/:id", UpdateUserHandler)
    r.Delete("/posts/:id", DeletePostHandler)
})
```

## URL Parameters

Named parameters are prefixed with `:`. Retrieve them with `c.Param("name")`.

```go
r.Get("/users/:username", func(c *onihttp.Context) error {
    username := c.Param("username")
    return c.JSON(200, map[string]any{"username": username})
})
```

## Route Groups

Groups share a prefix and optionally middleware.

```go
r.Group("/api/v1", func(g *routing.Group) {
    g.Get("/users", userCtrl.Index)
    g.Post("/users", userCtrl.Store)
    g.Get("/users/:id", userCtrl.Show)
})
```

## Middleware on Routes

Apply middleware to a single handler by wrapping it:

```go
authMW := middleware.Auth(guard)

r.Get("/dashboard", authMW(DashboardHandler))
```

Apply middleware to a group by calling `g.Use(...)`:

```go
r.Group("/admin", func(g *routing.Group) {
    g.Use(adminOnly)
    g.Get("/users", AdminUsersHandler)
})
```

## Global Middleware

Global middleware runs for every request:

```go
oni.Use(
    middleware.Logger(),
    middleware.Recovery(),
    middleware.CORS(),
)
```

## Wildcard Routes

Use `/*` to match any path segment (e.g. SPA catch-all):

```go
r.Get("/*", SPAHandler)
```

## Serving Static Files

```go
r.Get("/storage/*", func(c *onihttp.Context) error {
    http.ServeFile(c.Response, c.Request.Request, c.Request.URL.Path[1:])
    return nil
})
```

## Query Parameters

```go
page := c.Query("page")         // "" if missing
page := c.QueryD("page", "1")   // "1" if missing
```

## Validation (Bind + Validate in one step)

```go
func createUser(c *onihttp.Context) error {
    var in struct {
        Name  string `json:"name"  validate:"required,min=2"`
        Email string `json:"email" validate:"required,email"`
        Age   int    `json:"age"   validate:"gte=18"`
    }
    if err := c.Validate(&in); err != nil {
        return err // 422 {"message":"validation failed","errors":{...}}
    }
    // in.Name and in.Email are valid
    return c.JSON(201, in)
}
```

`c.Validate` binds the request body (JSON, XML, or form) and runs `validate` struct tag rules. Returns a structured `422` on failure automatically.

## Named Routes

```go
r.Get("/users/:id", getUser).Name("users.show")

// Generate URL
url, err := app.Router.URL("users.show", "id", "42")
// → /users/42
```

## Method Reference

| Method | Description |
|--------|-------------|
| `r.Get(path, handler)` | Register a GET route |
| `r.Post(path, handler)` | Register a POST route |
| `r.Put(path, handler)` | Register a PUT route |
| `r.Delete(path, handler)` | Register a DELETE route |
| `r.Patch(path, handler)` | Register a PATCH route |
| `r.Any(path, handler)` | Register all HTTP methods |
| `r.Group(prefix, fn)` | Create a route group with a shared prefix |
| `g.Use(mw...)` | Add middleware to a group |
| `c.Param("name")` | Read a URL path parameter |
| `c.Query("name")` | Read a URL query parameter |
| `c.QueryD("name", "default")` | Read query param with fallback |
| `c.Validate(&in)` | Bind + validate request body, returns 422 on failure |
| `c.Abort(code, msg)` | Return an HTTP error and stop the chain |
