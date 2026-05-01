# Authentication

OniWorks ships with a dual-mode auth guard: **session-based** (traditional web) and **JWT-based** (stateless APIs). Both are in `framework/auth`.

## User Interface

Your application's User model must implement `auth.User`:

```go
type User interface {
    GetID() int64
    GetEmail() string
    GetPassword() string  // bcrypt hash
}
```

Example:

```go
type User struct {
    ID           int64  `db:"id"`
    Email        string `db:"email"`
    PasswordHash string `db:"password_hash"`
}

func (u *User) GetID() int64        { return u.ID }
func (u *User) GetEmail() string    { return u.Email }
func (u *User) GetPassword() string { return u.PasswordHash }
```

## Setting Up the Guard

```go
import "github.com/onipixel/oniworks/framework/auth"

guard := auth.NewGuard(userProvider, sessionManager, jwtSecret)
```

For JWT-only apps, `userProvider` and `sessionManager` can be `nil`.

## Password Hashing

```go
// Hash a password before storing it
hash, err := auth.HashPassword(plaintext)

// Verify at login
ok := auth.CheckPassword(storedHash, plaintext)
```

## JWT Authentication (API)

### Issuing Tokens

```go
// Issue a token valid for 7 days
token, err := guard.IssueToken(user, 7 * 24 * time.Hour)
```

### Validating Tokens

```go
claims, err := guard.ParseToken(tokenString)
if err != nil {
    // auth.ErrInvalidToken if expired/malformed
}
fmt.Println(claims.UserID, claims.Email)
```

### Auth Middleware

Write a middleware that reads the `Authorization: Bearer <token>` header:

```go
func Auth(guard *auth.Guard) onihttp.MiddlewareFunc {
    return func(next onihttp.HandlerFunc) onihttp.HandlerFunc {
        return func(c *onihttp.Context) error {
            token := c.Request.BearerToken()
            if token == "" {
                return c.Abort(401, "unauthenticated")
            }
            claims, err := guard.ParseToken(token)
            if err != nil {
                return c.Abort(401, "invalid or expired token")
            }
            c.Set("user_id", claims.UserID)
            return next(c)
        }
    }
}
```

Apply it to a route:

```go
r.Get("/api/me", authMW(meHandler))
```

Read the user ID in a handler:

```go
uid, _ := c.Get("user_id")
userID := uid.(int64)
```

### Login Flow (full example)

```go
func (ctrl *AuthController) Login(c *onihttp.Context) error {
    var req struct {
        Email    string `json:"email"`
        Password string `json:"password"`
    }
    c.Bind(&req)

    var user models.User
    if err := database.Table("users").Where("email = ?", req.Email).First(&user); err != nil {
        return c.Abort(401, "invalid credentials")
    }
    if !auth.CheckPassword(user.PasswordHash, req.Password) {
        return c.Abort(401, "invalid credentials")
    }

    token, _ := guard.IssueToken(&user, 7*24*time.Hour)
    return c.JSON(200, map[string]any{"token": token, "user": user})
}
```

## Session Authentication (Web)

For traditional server-rendered applications use the session guard:

```go
// Login
user, err := guard.Attempt(ctx, email, password, sess)

// Check if logged in
if guard.Check(ctx, sess) { ... }

// Get current user
user, err := guard.UserFromSession(ctx, sess)

// Logout
guard.Logout(sess)
```

## Errors

| Error | Meaning |
|-------|---------|
| `auth.ErrInvalidCredentials` | Wrong email or password |
| `auth.ErrInvalidToken` | JWT is malformed or expired |
| `auth.ErrUnauthenticated` | No authenticated user found |
