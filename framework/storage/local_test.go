package storage_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/onipixel/oniworks/framework/storage"
)

func newLocalDisk(t *testing.T) *storage.Local {
	t.Helper()
	root := t.TempDir()
	disk, err := storage.NewLocal(root, "https://cdn.example.com")
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	return disk
}

func TestPutAndGet(t *testing.T) {
	disk := newLocalDisk(t)
	ctx := context.Background()

	content := "Hello, OniWorks Storage!"
	if err := disk.Put(ctx, "test/hello.txt", strings.NewReader(content)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	r, err := disk.Get(ctx, "test/hello.txt")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer r.Close()

	got, _ := io.ReadAll(r)
	if string(got) != content {
		t.Errorf("content: got %q, want %q", string(got), content)
	}
}

func TestExists(t *testing.T) {
	disk := newLocalDisk(t)
	ctx := context.Background()

	ok, err := disk.Exists(ctx, "nonexistent.txt")
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if ok {
		t.Error("nonexistent file should not exist")
	}

	_ = disk.Put(ctx, "file.txt", strings.NewReader("data"))
	ok, err = disk.Exists(ctx, "file.txt")
	if err != nil {
		t.Fatalf("Exists after put: %v", err)
	}
	if !ok {
		t.Error("file should exist after put")
	}
}

func TestDelete(t *testing.T) {
	disk := newLocalDisk(t)
	ctx := context.Background()

	_ = disk.Put(ctx, "delete-me.txt", strings.NewReader("bye"))
	if err := disk.Delete(ctx, "delete-me.txt"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	ok, _ := disk.Exists(ctx, "delete-me.txt")
	if ok {
		t.Error("deleted file should not exist")
	}
	// Deleting non-existent file should not error
	if err := disk.Delete(ctx, "already-gone.txt"); err != nil {
		t.Errorf("deleting non-existent: %v", err)
	}
}

func TestSize(t *testing.T) {
	disk := newLocalDisk(t)
	ctx := context.Background()

	data := []byte("exactly twenty chars")
	_ = disk.Put(ctx, "sized.txt", bytes.NewReader(data))

	n, err := disk.Size(ctx, "sized.txt")
	if err != nil {
		t.Fatalf("Size: %v", err)
	}
	if n != int64(len(data)) {
		t.Errorf("size: got %d, want %d", n, len(data))
	}
}

func TestList(t *testing.T) {
	disk := newLocalDisk(t)
	ctx := context.Background()

	_ = disk.Put(ctx, "dir/a.txt", strings.NewReader("a"))
	_ = disk.Put(ctx, "dir/b.txt", strings.NewReader("b"))
	_ = disk.Put(ctx, "other/c.txt", strings.NewReader("c"))

	keys, err := disk.List(ctx, "dir")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys under dir/, got %d: %v", len(keys), keys)
	}
}

func TestCopy(t *testing.T) {
	disk := newLocalDisk(t)
	ctx := context.Background()

	_ = disk.Put(ctx, "original.txt", strings.NewReader("copy me"))
	if err := disk.Copy(ctx, "original.txt", "copy.txt"); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	ok, _ := disk.Exists(ctx, "copy.txt")
	if !ok {
		t.Error("copied file should exist")
	}
	ok, _ = disk.Exists(ctx, "original.txt")
	if !ok {
		t.Error("original should still exist after copy")
	}
}

func TestMove(t *testing.T) {
	disk := newLocalDisk(t)
	ctx := context.Background()

	_ = disk.Put(ctx, "source.txt", strings.NewReader("move me"))
	if err := disk.Move(ctx, "source.txt", "dest.txt"); err != nil {
		t.Fatalf("Move: %v", err)
	}

	ok, _ := disk.Exists(ctx, "dest.txt")
	if !ok {
		t.Error("destination should exist after move")
	}
	ok, _ = disk.Exists(ctx, "source.txt")
	if ok {
		t.Error("source should not exist after move")
	}
}

func TestURL(t *testing.T) {
	disk := newLocalDisk(t)
	url := disk.URL("avatars/user-1.jpg")
	if !strings.Contains(url, "cdn.example.com") {
		t.Errorf("URL: got %q, expected CDN URL", url)
	}
	if !strings.Contains(url, "avatars/user-1.jpg") {
		t.Errorf("URL: should contain path, got %q", url)
	}
}

func TestSignedURL(t *testing.T) {
	disk := newLocalDisk(t)
	ctx := context.Background()
	url, err := disk.SignedURL(ctx, "private/file.pdf", 0)
	if err != nil {
		t.Fatalf("SignedURL: %v", err)
	}
	if url == "" {
		t.Error("SignedURL should not be empty")
	}
}

func TestGetMissingFile(t *testing.T) {
	disk := newLocalDisk(t)
	_, err := disk.Get(context.Background(), "does-not-exist.txt")
	if err == nil {
		t.Error("Get on missing file should return error")
	}
	if !os.IsNotExist(err) && !strings.Contains(err.Error(), "open") {
		t.Logf("error type: %v", err)
	}
}
