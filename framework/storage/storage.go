// Package storage provides a unified file storage abstraction with local-disk
// and S3-compatible (AWS S3, MinIO, Wasabi, R2) drivers.
package storage

import (
	"context"
	"io"
	"time"
)

// Disk is the interface every storage driver must implement.
type Disk interface {
	// Put writes r to the given path, creating directories as needed.
	Put(ctx context.Context, path string, r io.Reader, opts ...PutOptions) error
	// Get returns a reader for the object at path.
	Get(ctx context.Context, path string) (io.ReadCloser, error)
	// Delete removes the object at path.
	Delete(ctx context.Context, path string) error
	// Exists reports whether an object exists at path.
	Exists(ctx context.Context, path string) (bool, error)
	// URL returns the public URL for path. For signed URLs, use SignedURL.
	URL(path string) string
	// SignedURL returns a pre-signed URL valid for ttl.
	SignedURL(ctx context.Context, path string, ttl time.Duration) (string, error)
	// List returns the keys under prefix.
	List(ctx context.Context, prefix string) ([]string, error)
	// Size returns the byte size of the object at path.
	Size(ctx context.Context, path string) (int64, error)
	// Copy duplicates src to dst within the same disk.
	Copy(ctx context.Context, src, dst string) error
	// Move renames src to dst within the same disk.
	Move(ctx context.Context, src, dst string) error
}

// PutOptions holds optional metadata for a Put operation.
type PutOptions struct {
	ContentType string
	ACL         string // e.g. "public-read", "private"
	Metadata    map[string]string
}

// Manager holds named disk instances.
type Manager struct {
	disks   map[string]Disk
	Default string
}

// NewManager creates a Manager with the given named disks.
func NewManager(defaultDisk string, disks map[string]Disk) *Manager {
	return &Manager{disks: disks, Default: defaultDisk}
}

// Disk returns a named disk, or panics if not found.
func (m *Manager) Disk(name string) Disk {
	d, ok := m.disks[name]
	if !ok {
		panic("storage: unknown disk " + name)
	}
	return d
}

// Put writes to the default disk.
func (m *Manager) Put(ctx context.Context, path string, r io.Reader, opts ...PutOptions) error {
	return m.Disk(m.Default).Put(ctx, path, r, opts...)
}

// Get reads from the default disk.
func (m *Manager) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	return m.Disk(m.Default).Get(ctx, path)
}

// Delete deletes from the default disk.
func (m *Manager) Delete(ctx context.Context, path string) error {
	return m.Disk(m.Default).Delete(ctx, path)
}

// URL returns the public URL for a path on the default disk.
func (m *Manager) URL(path string) string {
	return m.Disk(m.Default).URL(path)
}
