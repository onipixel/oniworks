package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Local is a local-disk storage driver.
type Local struct {
	root    string // absolute path to the storage root
	baseURL string // public base URL, e.g. "https://example.com/storage"
}

// NewLocal creates a local disk driver.
//
//	disk := storage.NewLocal("/var/app/storage", "https://example.com/storage")
func NewLocal(root, baseURL string) (*Local, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	return &Local{root: abs, baseURL: strings.TrimRight(baseURL, "/")}, nil
}

// abs resolves a caller-supplied key to an absolute path inside the storage
// root, returning an error if the key would escape it. The leading-slash + Clean
// step neutralizes "../" on POSIX, and ToSlash normalizes Windows separators;
// the filepath.Rel containment check is the cross-platform backstop that also
// rejects absolute/volume-qualified (e.g. "C:\…", "\\host\share") inputs.
func (l *Local) abs(path string) (string, error) {
	clean := filepath.Clean("/" + filepath.ToSlash(path))
	joined := filepath.Join(l.root, clean)
	rel, err := filepath.Rel(l.root, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("storage/local: path %q escapes the storage root", path)
	}
	return joined, nil
}

func (l *Local) Put(_ context.Context, path string, r io.Reader, _ ...PutOptions) error {
	dst, err := l.abs(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("storage/local: mkdir: %w", err)
	}
	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("storage/local: create: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("storage/local: write: %w", err)
	}
	return nil
}

func (l *Local) Get(_ context.Context, path string) (io.ReadCloser, error) {
	p, err := l.abs(path)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("storage/local: open %q: %w", path, err)
	}
	return f, nil
}

func (l *Local) Delete(_ context.Context, path string) error {
	p, err := l.abs(path)
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (l *Local) Exists(_ context.Context, path string) (bool, error) {
	p, err := l.abs(path)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func (l *Local) URL(path string) string {
	return l.baseURL + "/" + strings.TrimLeft(filepath.ToSlash(path), "/")
}

// SignedURL is not meaningful for local disk; returns a plain URL.
func (l *Local) SignedURL(_ context.Context, path string, _ time.Duration) (string, error) {
	return l.URL(path), nil
}

func (l *Local) List(_ context.Context, prefix string) ([]string, error) {
	root, err := l.abs(prefix)
	if err != nil {
		return nil, err
	}
	var keys []string
	werr := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(l.root, p)
		keys = append(keys, filepath.ToSlash(rel))
		return nil
	})
	return keys, werr
}

func (l *Local) Size(_ context.Context, path string) (int64, error) {
	p, err := l.abs(path)
	if err != nil {
		return 0, err
	}
	fi, err := os.Stat(p)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

func (l *Local) Copy(ctx context.Context, src, dst string) error {
	r, err := l.Get(ctx, src)
	if err != nil {
		return err
	}
	defer r.Close()
	return l.Put(ctx, dst, r)
}

func (l *Local) Move(ctx context.Context, src, dst string) error {
	if err := l.Copy(ctx, src, dst); err != nil {
		return err
	}
	return l.Delete(ctx, src)
}
