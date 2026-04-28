package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"
)

// ─────────────────────────── oni new ──────────────────────────────

var newCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new OniWorks application",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		frontend, _ := cmd.Flags().GetBool("frontend")
		return scaffoldNew(args[0], frontend)
	},
}

func init() {
	newCmd.Flags().Bool("frontend", false, "include Vite + TypeScript + Tailwind CSS frontend")
}

// ─────────────────────────── oni serve ────────────────────────────

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the development server (hot-reload via Air if available)",
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetString("port")
		fmt.Printf("Starting OniWorks dev server on :%s\n", port)
		if airPath, err := exec.LookPath("air"); err == nil {
			c := exec.Command(airPath)
			c.Stdout, c.Stderr = os.Stdout, os.Stderr
			return c.Run()
		}
		c := exec.Command("go", "run", ".")
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		return c.Run()
	},
}

func init() { serveCmd.Flags().StringP("port", "p", "8080", "port to listen on") }

// ─────────────────────────── oni build ────────────────────────────

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Compile the production binary",
	RunE: func(cmd *cobra.Command, args []string) error {
		out, _ := cmd.Flags().GetString("output")
		if out == "" {
			wd, _ := os.Getwd()
			out = filepath.Base(wd)
			if runtime.GOOS == "windows" {
				out += ".exe"
			}
		}
		fmt.Printf("Building → %s\n", out)
		c := exec.Command("go", "build", "-ldflags=-s -w", "-o", out, ".")
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		return c.Run()
	},
}

func init() { buildCmd.Flags().StringP("output", "o", "", "output binary path") }

// ─────────────────────────── oni deploy ───────────────────────────

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy with Caddy + automatic Let's Encrypt TLS",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Starting Oni Deploy (Caddy + Let's Encrypt)...")
		return runAppCommand("deploy")
	},
}

// ─────────────────────────── migrations ───────────────────────────

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run all pending migrations",
	RunE:  func(cmd *cobra.Command, args []string) error { return runAppCommand("migrate") },
}

var migrateRollbackCmd = &cobra.Command{
	Use:   "migrate:rollback",
	Short: "Roll back the last migration batch",
	RunE:  func(cmd *cobra.Command, args []string) error { return runAppCommand("migrate:rollback") },
}

var migrateFreshCmd = &cobra.Command{
	Use:   "migrate:fresh",
	Short: "Drop all tables and re-run all migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print("WARNING: This will drop ALL tables. Type 'yes' to continue: ")
		var answer string
		fmt.Scan(&answer)
		if strings.TrimSpace(answer) != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
		return runAppCommand("migrate:fresh")
	},
}

var migrateStatusCmd = &cobra.Command{
	Use:   "migrate:status",
	Short: "Show the status of each migration",
	RunE:  func(cmd *cobra.Command, args []string) error { return runAppCommand("migrate:status") },
}

// ─────────────────────────── db:seed ──────────────────────────────

var dbSeedCmd = &cobra.Command{
	Use:   "db:seed",
	Short: "Run database seeders",
	RunE: func(cmd *cobra.Command, args []string) error {
		class, _ := cmd.Flags().GetString("class")
		sub := "db:seed"
		if class != "" {
			sub += ":" + class
		}
		return runAppCommand(sub)
	},
}

func init() { dbSeedCmd.Flags().String("class", "", "run a specific seeder class") }

// ─────────────────────────── route:list ───────────────────────────

var routeListCmd = &cobra.Command{
	Use:   "route:list",
	Short: "List all registered routes",
	RunE:  func(cmd *cobra.Command, args []string) error { return runAppCommand("route:list") },
}

// ─────────────────────────── key:generate ─────────────────────────

var keyGenerateCmd = &cobra.Command{
	Use:   "key:generate",
	Short: "Generate and set a new APP_KEY",
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := generateKey(32)
		if err != nil {
			return err
		}
		fmt.Printf("APP_KEY=%s\n", key)
		updateEnvKey(".env", "APP_KEY", key)
		fmt.Println("APP_KEY updated in .env")
		return nil
	},
}

// ─────────────────────────── admin:install ────────────────────────

var adminInstallCmd = &cobra.Command{
	Use:   "admin:install",
	Short: "Install the Oni Admin panel",
	RunE:  func(cmd *cobra.Command, args []string) error { return runAppCommand("admin:install") },
}

// ─────────────────────────── queue:* ──────────────────────────────

var queueWorkCmd = &cobra.Command{
	Use:   "queue:work",
	Short: "Start processing queue jobs",
	RunE: func(cmd *cobra.Command, args []string) error {
		q, _ := cmd.Flags().GetString("queue")
		tries, _ := cmd.Flags().GetInt("tries")
		return runAppCommand(fmt.Sprintf("queue:work queue=%s tries=%d", q, tries))
	},
}

func init() {
	queueWorkCmd.Flags().String("queue", "default", "queue name")
	queueWorkCmd.Flags().Int("tries", 3, "max attempts per job")
}

var queueRestartCmd = &cobra.Command{
	Use:   "queue:restart",
	Short: "Gracefully restart all queue workers",
	RunE:  func(cmd *cobra.Command, args []string) error { return runAppCommand("queue:restart") },
}

// ─────────────────────────── schedule:run ─────────────────────────

var scheduleRunCmd = &cobra.Command{
	Use:   "schedule:run",
	Short: "Run all due scheduled tasks",
	RunE:  func(cmd *cobra.Command, args []string) error { return runAppCommand("schedule:run") },
}

// ─────────────────────────── backup / restore ─────────────────────

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup the database to storage",
	RunE:  func(cmd *cobra.Command, args []string) error { return runAppCommand("backup") },
}

var restoreCmd = &cobra.Command{
	Use:   "restore <file>",
	Short: "Restore the database from a backup file",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return runAppCommand("restore:" + args[0]) },
}

// ─────────────────────────── health ───────────────────────────────

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Run application health checks",
	RunE:  func(cmd *cobra.Command, args []string) error { return runAppCommand("health") },
}

// ─────────────────────────── docs:serve ───────────────────────────

var docsServeCmd = &cobra.Command{
	Use:   "docs:serve",
	Short: "Serve OniWorks documentation locally",
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetString("port")
		fmt.Printf("Docs available at http://localhost:%s\n", port)
		return runAppCommand("docs:serve port=" + port)
	},
}

func init() { docsServeCmd.Flags().String("port", "4000", "port for docs server") }

// ─────────────────────────── secrets:* ────────────────────────────

var secretsGroup = &cobra.Command{Use: "secrets", Short: "Manage encrypted secrets"}

var secretsSetCmd = &cobra.Command{
	Use:   "secrets:set <KEY> <VALUE>",
	Short: "Encrypt and store a secret",
	Args:  cobra.ExactArgs(2),
	RunE:  func(cmd *cobra.Command, args []string) error { return runAppCommand("secrets:set " + args[0] + " " + args[1]) },
}

var secretsGetCmd = &cobra.Command{
	Use:   "secrets:get <KEY>",
	Short: "Decrypt and print a secret",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return runAppCommand("secrets:get " + args[0]) },
}

func init() { secretsGroup.AddCommand(secretsSetCmd, secretsGetCmd) }

// ─────────────────────────── make:* ───────────────────────────────

var makeGroup = &cobra.Command{Use: "make", Short: "Generate application boilerplate"}

var makeControllerCmd = &cobra.Command{
	Use:   "make:controller <Name>",
	Short: "Generate a controller",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return makeStub("controller", args[0]) },
}

var makeModelCmd = &cobra.Command{
	Use:   "make:model <Name>",
	Short: "Generate a model",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m, _ := cmd.Flags().GetBool("migration")
		if err := makeStub("model", args[0]); err != nil {
			return err
		}
		if m {
			return makeStub("migration", "create_"+toSnakeCase(args[0])+"s_table")
		}
		return nil
	},
}

func init() { makeModelCmd.Flags().BoolP("migration", "m", false, "also create a migration") }

var makeMigrationCmd = &cobra.Command{
	Use:  "make:migration <name>",
	Short: "Generate a blank migration",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error { return makeStub("migration", args[0]) },
}

var makeMiddlewareCmd = &cobra.Command{
	Use:   "make:middleware <Name>",
	Short: "Generate a middleware",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return makeStub("middleware", args[0]) },
}

var makeJobCmd = &cobra.Command{
	Use:   "make:job <Name>",
	Short: "Generate a queue job",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return makeStub("job", args[0]) },
}

var makeMailCmd = &cobra.Command{
	Use:   "make:mail <Name>",
	Short: "Generate a mailable",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return makeStub("mail", args[0]) },
}

var makeSeederCmd = &cobra.Command{
	Use:   "make:seeder <Name>",
	Short: "Generate a seeder",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return makeStub("seeder", args[0]) },
}

var makePolicyCmd = &cobra.Command{
	Use:   "make:policy <Name>",
	Short: "Generate an RBAC policy",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return makeStub("policy", args[0]) },
}

var makeTestCmd = &cobra.Command{
	Use:   "make:test <Name>",
	Short: "Generate a test file",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return makeStub("test", args[0]) },
}

var makeChannelCmd = &cobra.Command{
	Use:   "make:channel <Name>",
	Short: "Generate a realtime channel handler",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return makeStub("channel", args[0]) },
}

var makeResourceCmd = &cobra.Command{
	Use:   "make:resource <Name>",
	Short: "Generate a controller, model, and migration together",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		for _, kind := range []string{"controller", "model"} {
			if err := makeStub(kind, name); err != nil {
				return err
			}
		}
		return makeStub("migration", "create_"+toSnakeCase(name)+"s_table")
	},
}

func init() {
	makeGroup.AddCommand(
		makeControllerCmd, makeModelCmd, makeMigrationCmd,
		makeMiddlewareCmd, makeJobCmd, makeMailCmd,
		makeSeederCmd, makePolicyCmd, makeTestCmd,
		makeChannelCmd, makeResourceCmd,
	)
}

// ─────────────────────────── helpers ──────────────────────────────

func runAppCommand(cmd string) error {
	fmt.Printf("[oni] → %s\n", cmd)
	if _, err := os.Stat("main.go"); err == nil {
		c := exec.Command("go", "run", ".", "--oni-cmd="+cmd)
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		return c.Run()
	}
	return fmt.Errorf("main.go not found — are you inside an OniWorks project?")
}

func makeStub(kind, name string) error {
	stubPath := filepath.Join("stubs", kind+".stub")
	content, err := os.ReadFile(stubPath)
	if err != nil {
		return fmt.Errorf("stub not found: %s/stubs/%s.stub", ".", kind)
	}
	tmpl, err := template.New(kind).Parse(string(content))
	if err != nil {
		return err
	}
	data := map[string]any{
		"Name":      toPascalCase(name),
		"NameSnake": toSnakeCase(name),
		"Timestamp": time.Now().Format("20060102150405"),
		"Date":      time.Now().Format("2006-01-02"),
	}
	dir, filename := stubOutputPath(kind, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	outPath := filepath.Join(dir, filename)
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := tmpl.Execute(f, data); err != nil {
		return err
	}
	fmt.Printf("Created: %s\n", outPath)
	return nil
}

func stubOutputPath(kind, name string) (dir, filename string) {
	snake := toSnakeCase(name)
	ts := time.Now().Format("20060102150405")
	switch kind {
	case "controller":
		return filepath.Join("app", "http", "controllers"), snake + "_controller.go"
	case "model":
		return filepath.Join("app", "models"), snake + ".go"
	case "migration":
		return filepath.Join("database", "migrations"), ts + "_" + snake + ".go"
	case "middleware":
		return filepath.Join("app", "http", "middleware"), snake + ".go"
	case "job":
		return filepath.Join("app", "jobs"), snake + "_job.go"
	case "mail":
		return filepath.Join("app", "mail"), snake + "_mail.go"
	case "seeder":
		return filepath.Join("database", "seeders"), snake + "_seeder.go"
	case "policy":
		return filepath.Join("app", "policies"), snake + "_policy.go"
	case "test":
		return "tests", snake + "_test.go"
	default:
		return ".", snake + ".go"
	}
}

func scaffoldNew(name string, frontend bool) error {
	dirs := []string{
		"app/http/controllers", "app/http/middleware",
		"app/models", "app/jobs", "app/mail", "app/policies",
		"config", "database/migrations", "database/seeders",
		"storage/app", "storage/logs", "tests", "public",
	}
	if frontend {
		dirs = append(dirs, "resources/ts", "resources/css", "resources/views")
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(name, d), 0755); err != nil {
			return err
		}
	}
	key, _ := generateKey(32)
	writeProjectFile(name, "main.go", mainGoStub, map[string]any{"Name": name})
	writeProjectFile(name, ".env", envStub, map[string]any{"AppKey": key, "Name": name})
	writeProjectFile(name, "config/app.yaml", appYamlStub, map[string]any{"Name": name})
	writeProjectFile(name, ".gitignore", gitignoreStub, nil)

	if frontend {
		writeProjectFile(name, "vite.config.ts", viteConfigStub, nil)
		writeProjectFile(name, "tailwind.config.ts", tailwindConfigStub, nil)
		writeProjectFile(name, "resources/css/app.css", tailwindCSSStub, nil)
		writeProjectFile(name, "resources/ts/app.ts", appTSStub, nil)
		writeProjectFile(name, "package.json", packageJSONStub, map[string]any{"Name": name})
		writeProjectFile(name, "tsconfig.json", tsconfigStub, nil)
	}

	fmt.Printf("\nOniWorks app %q created!\n\n", name)
	fmt.Printf("  cd %s\n", name)
	fmt.Println("  go mod init your.module/" + name)
	fmt.Println("  go get github.com/oniworks/oniworks")
	fmt.Println("  oni serve\n")
	return nil
}

func writeProjectFile(root, path, tmplStr string, data map[string]any) {
	full := filepath.Join(root, path)
	_ = os.MkdirAll(filepath.Dir(full), 0755)
	tmpl, err := template.New("").Parse(tmplStr)
	if err != nil {
		return
	}
	f, err := os.Create(full)
	if err != nil {
		return
	}
	defer f.Close()
	_ = tmpl.Execute(f, data)
}

func generateKey(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func updateEnvKey(envFile, key, value string) {
	data, err := os.ReadFile(envFile)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			lines[i] = key + "=" + value
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, key+"="+value)
	}
	_ = os.WriteFile(envFile, []byte(strings.Join(lines, "\n")), 0644)
}

func toPascalCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' || r == ' ' })
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

func toSnakeCase(s string) string {
	var out strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				out.WriteByte('_')
			}
			out.WriteRune(r + 32)
		} else {
			out.WriteRune(r)
		}
	}
	return strings.ReplaceAll(out.String(), "-", "_")
}

// ─────────────────────── scaffold templates ───────────────────────

const mainGoStub = `package main

import (
	"github.com/oniworks/oniworks/framework/app"
	"github.com/oniworks/oniworks/framework/middleware"
	onihttp "github.com/oniworks/oniworks/framework/http"
	"github.com/oniworks/oniworks/framework/routing"
)

func main() {
	oni := app.New()
	oni.Load(".env", "config/app.yaml")

	oni.Use(
		middleware.Logger(),
		middleware.Recovery(),
		middleware.CORS(),
	)

	oni.Route(func(r *routing.Router) {
		r.Get("/", func(c *onihttp.Context) error {
			return c.JSON(200, map[string]any{
				"message": "Welcome to {{.Name}}!",
				"powered": "OniWorks",
			})
		})

		r.Group("/api/v1", func(g *routing.Group) {
			// r.Use(middleware.Auth())
			// g.Get("/users", UserController.Index)
		})
	})

	oni.Serve()
}
`

const envStub = `APP_NAME={{.Name}}
APP_ENV=local
APP_DEBUG=true
APP_KEY={{.AppKey}}
APP_URL=http://localhost:8080

DB_DRIVER=postgres
DB_HOST=127.0.0.1
DB_PORT=5432
DB_NAME={{.Name}}
DB_USER=postgres
DB_PASSWORD=

QUEUE_DRIVER=memory
MAIL_DRIVER=smtp
MAIL_HOST=localhost
MAIL_PORT=1025
`

const appYamlStub = `app:
  name: "{{.Name}}"
  env: local
  debug: true
  url: "http://localhost:8080"

server:
  host: ""
  port: 8080

database:
  driver: postgres
  host: "127.0.0.1"
  port: 5432
  name: "{{.Name}}"
  user: postgres
  password: ""
  pool:
    max_open: 25
    max_idle: 5

queue:
  driver: memory

mail:
  driver: smtp
  host: localhost
  port: 1025
`

const gitignoreStub = `.env.production
*.exe
storage/logs/*.log
storage/memory.snap
public/build/
node_modules/
`

const viteConfigStub = `import { defineConfig } from 'vite'

export default defineConfig({
  root: 'resources',
  build: {
    outDir: '../public/build',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/ws':  { target: 'ws://localhost:8080', ws: true },
    },
  },
})
`

const tailwindConfigStub = `import type { Config } from 'tailwindcss'
export default {
  content: ['./resources/**/*.{ts,tsx,html}'],
  theme: { extend: {} },
  plugins: [],
} satisfies Config
`

const tailwindCSSStub = `@tailwind base;
@tailwind components;
@tailwind utilities;
`

const appTSStub = `import '../css/app.css'
console.log('OniWorks ready')
`

const packageJSONStub = `{
  "name": "{{.Name}}",
  "private": true,
  "scripts": {
    "dev": "vite",
    "build": "vite build"
  },
  "devDependencies": {
    "vite": "^6.0.0",
    "typescript": "^5.0.0",
    "tailwindcss": "^4.0.0"
  }
}
`

const tsconfigStub = `{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "skipLibCheck": true
  },
  "include": ["resources/ts/**/*"]
}
`
