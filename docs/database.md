# Database & ORM

OniWorks includes a performant query builder with struct scanning, lifecycle hooks, and explicit eager loading. It is designed to be fast and explicit — no lazy loading, no N+1 queries by design.

## Connection

```go
db, err := database.Open(database.Config{
    Driver:   database.DriverPostgres,  // or database.DriverMySQL
    Host:     "127.0.0.1",
    Port:     5432,
    Name:     "myapp",
    User:     "postgres",
    Password: "secret",
    SSLMode:  "disable",
    MaxOpen:  25,
    MaxIdle:  5,
})
// Set as the package-level default so controllers can use database.Table(...)
database.SetDefault(db)
```

## Query Builder

```go
// Select all
var users []User
err := db.Table("users").All(&users)

// Where + limit
var user User
err := db.Table("users").
    Where("email = ?", email).
    First(&user)

// Multiple conditions
var posts []Post
err := db.Table("posts").
    Where("user_id = ?", userID).
    Where("published = ?", true).
    OrderBy("created_at DESC").
    Limit(10).
    Offset(0).
    All(&posts)

// Raw query
var count int
err := db.Raw("SELECT COUNT(*) FROM users WHERE active = ?", true).
    Scan(&count)
```

## Insert

```go
user := &User{Email: "alice@example.com", Name: "Alice"}
err := db.Table("users").Insert(user)
// user.ID is set after insert (for auto-increment PKs)
```

## Update

```go
err := db.Table("users").
    Where("id = ?", 1).
    Update(database.Map{"name": "Bob"})

// Or update a struct (only non-zero fields)
user.Name = "Bob"
err := db.Table("users").Save(&user)
```

## Delete

```go
err := db.Table("users").Where("id = ?", id).Delete()
```

## Aggregates

```go
count, err := db.Table("users").Where("active = ?", true).Count()
exists, err := db.Table("users").Where("email = ?", email).Exists()

// Pluck retrieves a single column into a typed slice
var emails []string
err = db.Table("users").Pluck("email", &emails)
```

## Eager Loading (Batch, No N+1)

OniWorks eager loads relationships in **one SQL query per relation** using `IN` clauses:

```go
var posts []Post
err := db.Table("posts").
    With("author", "comments").   // HasMany, BelongsTo
    All(&posts)
// Runs: SELECT * FROM posts
// Then: SELECT * FROM users WHERE id IN (...)
// Then: SELECT * FROM comments WHERE post_id IN (...)
```

> There is no lazy loading. Relations must be explicitly loaded with `With()`.

## Transactions

```go
err := db.Transaction(func(tx *database.DB) error {
    // Use raw SQL expressions for arithmetic updates
    if err := tx.Table("accounts").Where("id = ?", from).
        Update(database.Map{"balance": tx.Raw("balance - ?", amount)}); err != nil {
        return err  // auto-rollback
    }
    return tx.Table("accounts").Where("id = ?", to).
        Update(database.Map{"balance": tx.Raw("balance + ?", amount)})
})
// Note: for simple arithmetic, use db.Raw() to build an expression:
//   db.Table("posts").Where("id = ?", id).Update(database.Map{"view_count": "view_count + 1"})
```

## Lifecycle Hooks

Implement any of these interfaces on your model:

```go
func (u *User) BeforeCreate() error { u.CreatedAt = time.Now(); return nil }
func (u *User) AfterCreate() error  { slog.Info("user created", "id", u.ID); return nil }
func (u *User) BeforeSave() error   { return u.validate() }
func (u *User) AfterFind() error    { u.FullName = u.FirstName + " " + u.LastName; return nil }
```

Full list: `BeforeCreate`, `AfterCreate`, `BeforeSave`, `AfterSave`, `BeforeUpdate`, `AfterUpdate`, `BeforeDelete`, `AfterDelete`, `AfterFind`.

## Migrations

```bash
oni make:migration create_users_table
```

```go
// Migration Up/Down do NOT return an error — they queue DDL statements.
// The Migrator executes them and handles errors internally.
func (m *Migration) Up(schema *migrations.Schema) {
    schema.Create("users", func(t *migrations.Table) {
        t.ID()
        t.String("name", 100)
        t.String("email", 255).Unique()
        t.String("password_hash", 255)
        t.Boolean("active").Default(true)
        t.Timestamps()
    })
}

func (m *Migration) Down(schema *migrations.Schema) {
    schema.DropIfExists("users")
}
```

Migrations are auto-registered via `init()` and discovered through a side-effect import in `main.go`:

```go
import _ "myapp/database/migrations"
```

```bash
oni migrate          # run
oni migrate:rollback # revert last batch
oni migrate:fresh    # drop all + re-run
oni migrate:status   # show status
```
