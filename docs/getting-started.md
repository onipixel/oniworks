# Getting Started

## Installation

OniWorks requires Go 1.22 or later.

```bash
go install github.com/oniworks/oniworks/cmd/oni@latest
```

## Create a Project

```bash
oni new my-app             # API-only
oni new my-app --frontend  # with Vite + TypeScript + Tailwind CSS
```

This scaffolds:

```
my-app/
├── main.go
├── .env
├── config/app.yaml
├── app/
│   ├── http/
│   │   ├── controllers/
│   │   └── middleware/
│   ├── models/
│   ├── jobs/
│   └── policies/
├── database/
│   ├── migrations/
│   └── seeders/
├── storage/
│   ├── app/
│   └── logs/
└── tests/
```

With `--frontend` you also get:

```
├── resources/
│   ├── ts/app.ts
│   └── css/app.css
├── vite.config.ts
├── tailwind.config.ts
├── tsconfig.json
└── package.json
```

## First App

```go
package main

import (
    "github.com/oniworks/oniworks/framework/app"
    onihttp "github.com/oniworks/oniworks/framework/http"
    "github.com/oniworks/oniworks/framework/middleware"
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
                "message": "Hello from OniWorks!",
            })
        })
    })

    oni.Serve()
}
```

## Development Server

```bash
oni serve            # uses Air for hot-reload if installed
go install github.com/air-verse/air@latest
```

## Environment

OniWorks loads `.env` and merges it with `config/app.yaml`:

```env
APP_NAME=MyApp
APP_ENV=local
APP_KEY=base64:your-32-byte-key-here
APP_URL=http://localhost:8080

DB_DRIVER=postgres
DB_HOST=127.0.0.1
DB_PORT=5432
DB_NAME=myapp
DB_USER=postgres
DB_PASSWORD=secret
```

Generate a secure APP_KEY:

```bash
oni key:generate
```
