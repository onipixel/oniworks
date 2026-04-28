package app

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/onipixel/oniworks/framework/config"
	onihttp "github.com/onipixel/oniworks/framework/http"
	"github.com/onipixel/oniworks/framework/routing"
)

// Application is the central OniWorks application object.
// It wires together the IoC container, config, router, and HTTP kernel.
type Application struct {
	*Container
	Config *config.Config
	Router *routing.Router
	Logger *slog.Logger

	kernel    *onihttp.Kernel
	providers []ServiceProvider
	booted    bool
}

// New creates and returns a new Application with sensible defaults.
func New() *Application {
	return &Application{
		Container: NewContainer(),
		Config:    config.New(),
		Router:    routing.New(),
		Logger:    slog.Default(),
	}
}

// Load reads one or more config files (YAML, TOML) and/or .env files.
// Files are loaded in order; later values override earlier ones.
//
//	oni.Load("config/app.yaml", ".env")
func (a *Application) Load(files ...string) *Application {
	for _, f := range files {
		ext := filepath.Ext(f)
		switch ext {
		case ".env", "":
			if err := config.LoadEnv(f); err != nil {
				a.Logger.Error("failed to load env file", "path", f, "error", err)
			}
		default:
			if err := config.Load(a.Config, f); err != nil {
				a.Logger.Error("failed to load config file", "path", f, "error", err)
			}
		}
	}
	return a
}

// LoadDir loads all YAML/TOML config files from a directory, keyed by filename.
//
//	oni.LoadDir("config/")  // loads config/app.yaml as "app.*", config/database.yaml as "database.*"
func (a *Application) LoadDir(dir string) *Application {
	if err := config.LoadDir(a.Config, dir); err != nil {
		a.Logger.Error("failed to load config directory", "dir", dir, "error", err)
	}
	return a
}

// Register adds one or more service providers. Register is called before Boot.
func (a *Application) Register(providers ...ServiceProvider) *Application {
	a.providers = append(a.providers, providers...)
	for _, p := range providers {
		p.Register(a)
	}
	return a
}

// Boot calls Boot() on all registered providers in registration order.
// Boot is called automatically by Serve; call it manually if you need services
// before starting the server (e.g. in CLI commands).
func (a *Application) Boot() *Application {
	if a.booted {
		return a
	}
	for _, p := range a.providers {
		p.Boot(a)
	}
	a.booted = true
	return a
}

// Route registers routes on the application router using a callback.
//
//	oni.Route(func(r *routing.Router) {
//	    r.Get("/", HomeHandler)
//	    r.Group("/api/v1", func(r *routing.Group) { ... })
//	})
func (a *Application) Route(fn func(*routing.Router)) *Application {
	fn(a.Router)
	return a
}

// Use appends global middleware to the application router.
func (a *Application) Use(mw ...onihttp.MiddlewareFunc) *Application {
	a.Router.Use(mw...)
	return a
}

// Serve starts the HTTP server. It blocks until shutdown.
func (a *Application) Serve() error {
	a.Boot()

	cfg := onihttp.ServerConfig{
		Host:         a.Config.String("server.host", ""),
		Port:         a.Config.Int("server.port", 8080),
		TLSCertFile:  a.Config.String("server.tls_cert", ""),
		TLSKeyFile:   a.Config.String("server.tls_key", ""),
	}

	a.kernel = onihttp.NewKernel(cfg, a.Router).WithLogger(a.Logger)
	a.Logger.Info("OniWorks ready", "addr", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port))
	return a.kernel.Serve()
}

// ServeAddr starts the server on the given address (overrides config).
// Useful for testing.
func (a *Application) ServeAddr(addr string) error {
	a.Boot()
	cfg := onihttp.DefaultServerConfig()
	cfg.Host = ""
	cfg.Port = 0

	a.kernel = onihttp.NewKernel(cfg, a.Router).WithLogger(a.Logger)
	return a.kernel.Serve()
}

// SetLogger replaces the application logger.
func (a *Application) SetLogger(l *slog.Logger) *Application {
	a.Logger = l
	slog.SetDefault(l)
	return a
}

// Env is a shortcut to read an environment variable with a fallback.
func (a *Application) Env(key, def string) string {
	return config.Env(key, def)
}

// IsDebug reports whether the application is running in debug mode.
func (a *Application) IsDebug() bool {
	return a.Config.Bool("app.debug", false) || os.Getenv("APP_DEBUG") == "true"
}

// IsProduction reports whether APP_ENV is "production".
func (a *Application) IsProduction() bool {
	return a.Config.String("app.env", os.Getenv("APP_ENV")) == "production"
}
