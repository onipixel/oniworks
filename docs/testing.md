# Testing

OniWorks ships with a fluent HTTP test client and fake drivers for mail, queues, and storage.

## HTTP Tests

```go
import (
    "testing"
    onitest "github.com/onipixel/oniworks/framework/testing"
)

func TestCreatePost(t *testing.T) {
    app := onitest.NewApp(t, myRouter)

    resp := app.POST("/api/v1/posts", map[string]any{
        "title":   "Hello OniWorks",
        "content": "This is a test post.",
    }).WithBearerToken("test-token").Send()

    resp.AssertCreated().
        AssertJSON("message", "created").
        AssertHeader("Content-Type", "application/json")
}

func TestGetPost(t *testing.T) {
    app := onitest.NewApp(t, myRouter)

    resp := app.GET("/api/v1/posts/1").Send()
    resp.AssertOK()

    var post map[string]any
    resp.JSON(&post)
    // assert post fields...
}

func TestUnauthorized(t *testing.T) {
    app := onitest.NewApp(t, myRouter)
    app.GET("/api/v1/posts").Send().AssertUnauthorized()
}
```

## Fake Mail

```go
func TestWelcomeEmail(t *testing.T) {
    fake := &onitest.FakeMail{}

    // Inject fake into your service
    svc := NewUserService(fake)
    svc.Register("alice@example.com", "password123")

    fake.AssertSent(t, "alice@example.com")

    msg, ok := fake.Last()
    if !ok || msg.Subject != "Welcome to OniWorks!" {
        t.Error("unexpected email")
    }
}
```

## Fake Queue

```go
func TestJobDispatched(t *testing.T) {
    fq := &onitest.FakeQueue{}
    svc := NewOrderService(fq)
    svc.PlaceOrder(order)

    fq.AssertDispatched(t, "ProcessOrderJob")
    fq.AssertNotDispatched(t, "CancelOrderJob")
}
```

## Fake Storage

```go
func TestAvatarUpload(t *testing.T) {
    fs := onitest.NewFakeStorage()
    svc := NewProfileService(fs)
    svc.UploadAvatar(1, bytes.NewReader(imageData))

    fs.AssertStored(t, "avatars/1.jpg")
}
```

## Test Assertions

| Method | Description |
|--------|-------------|
| `resp.AssertStatus(code)` | Assert HTTP status code |
| `resp.AssertOK()` | Assert 200 |
| `resp.AssertCreated()` | Assert 201 |
| `resp.AssertNotFound()` | Assert 404 |
| `resp.AssertUnauthorized()` | Assert 401 |
| `resp.AssertJSON(key, val)` | Assert JSON field value |
| `resp.AssertContains(str)` | Assert body contains string |
| `resp.AssertHeader(key, val)` | Assert response header |
| `resp.JSON(&v)` | Decode body into v |
| `resp.BodyString()` | Get body as string |

## Running Tests

```bash
go test ./...                    # all tests
go test ./tests/... -v           # verbose
go test ./tests/... -run TestAPI # specific test
```
