# Frontend Integration

OniWorks ships a `framework/frontend` package that manages Vite-built assets in both development and production.

## Scaffold with Frontend

```bash
oni new myapp --frontend
```

This creates:

```
myapp/
├── resources/
│   ├── ts/app.ts       (entry point)
│   └── css/app.css     (Tailwind CSS)
├── package.json
├── vite.config.ts
└── tsconfig.json
```

## Vite Configuration

The default `vite.config.ts` produced by `oni new --frontend`:

```typescript
import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  publicDir: false,  // important: prevents conflict with outDir
  plugins: [tailwindcss()],
  build: {
    outDir: 'public/build',
    emptyOutDir: true,
    manifest: true,
    rollupOptions: {
      input: { app: 'resources/ts/app.ts' },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/ws':  { target: 'ws://localhost:8080', ws: true },
    },
  },
})
```

> **Note**: Always set `publicDir: false` when `outDir` is inside the `public/` folder, otherwise Vite warns about overlapping directories.

## Asset Manager in Go

```go
import "github.com/onipixel/oniworks/framework/frontend"

fe := frontend.New("dev")                     // dev or production
fe.LoadManifest("public/build/.vite/manifest.json")  // production only
```

### Inject Vite Tags into HTML

```go
viteTag := fe.ViteTag("resources/ts/app.ts")
// In dev:  <script type="module" src="http://localhost:5173/resources/ts/app.ts"></script>
// In prod: <link rel="stylesheet" href="/assets/app-xxx.css">
//          <script type="module" src="/assets/app-xxx.js"></script>
```

### Serving a SPA Shell

```go
r.Get("/*", func(c *onihttp.Context) error {
    html := `<!doctype html>
<html>
<head>
  ` + fe.ViteTag("resources/ts/app.ts") + `
</head>
<body><div id="app"></div></body>
</html>`
    return c.HTML(200, html)
})
```

## Development Workflow

Start Go backend:
```bash
oni serve        # or: go run .
```

Start Vite dev server (in another terminal):
```bash
npm run dev
```

Visit `http://localhost:5173` — Vite proxies API calls to Go.

## Production Build

```bash
npm run build                # compiles to public/build/
oni build                    # compiles Go binary
```

Set `APP_ENV=production` so `frontend.New("production")` reads the manifest instead of proxying.

## TypeScript Type Generation

OniWorks can auto-generate TypeScript interfaces from your Go model structs:

```go
// In a CLI command or build step:
frontend.GenerateTypes("app/models", "resources/ts/types.generated.ts")
```

This scans Go structs with `json` tags and emits matching TypeScript `interface` declarations.

## Installing Dependencies

```bash
npm install
# or with specific packages:
npm install @tailwindcss/vite tailwindcss vite typescript
```

## Tailwind CSS

Tailwind v4 is configured via the `@tailwindcss/vite` plugin — no separate `tailwind.config.js` needed for basic use:

```css
/* resources/css/app.css */
@import "tailwindcss";
```
