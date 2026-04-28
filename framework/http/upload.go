package http

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// UploadedFile wraps a *multipart.FileHeader with helpers for inspection and saving.
type UploadedFile struct {
	Header   *multipart.FileHeader
	Original string   // original filename from client
	Size     int64    // bytes
	MIMEType string
}

// UploadConfig controls max file size and allowed MIME types.
type UploadConfig struct {
	MaxSize      int64    // bytes; default 10 MB
	AllowedTypes []string // e.g. ["image/jpeg","image/png"]; nil = all allowed
}

// DefaultUploadConfig returns sensible upload defaults.
func DefaultUploadConfig() UploadConfig {
	return UploadConfig{MaxSize: 10 << 20} // 10 MB
}

// ParseUpload parses a single file upload from the request field `name`.
func (c *Context) ParseUpload(field string, cfg ...UploadConfig) (*UploadedFile, error) {
	conf := DefaultUploadConfig()
	if len(cfg) > 0 {
		conf = cfg[0]
	}
	if err := c.Request.ParseMultipartForm(conf.MaxSize); err != nil {
		return nil, fmt.Errorf("upload: parse form: %w", err)
	}
	_, fh, err := c.Request.FormFile(field)
	if err != nil {
		return nil, fmt.Errorf("upload: field %q: %w", field, err)
	}
	return newUploadedFile(fh, conf)
}

// ParseUploads parses all files uploaded under a multi-value field name.
func (c *Context) ParseUploads(field string, cfg ...UploadConfig) ([]*UploadedFile, error) {
	conf := DefaultUploadConfig()
	if len(cfg) > 0 {
		conf = cfg[0]
	}
	if err := c.Request.ParseMultipartForm(conf.MaxSize); err != nil {
		return nil, fmt.Errorf("upload: parse form: %w", err)
	}
	fhs := c.Request.MultipartForm.File[field]
	if len(fhs) == 0 {
		return nil, fmt.Errorf("upload: no files in field %q", field)
	}
	files := make([]*UploadedFile, 0, len(fhs))
	for _, fh := range fhs {
		uf, err := newUploadedFile(fh, conf)
		if err != nil {
			return nil, err
		}
		files = append(files, uf)
	}
	return files, nil
}

func newUploadedFile(fh *multipart.FileHeader, cfg UploadConfig) (*UploadedFile, error) {
	if cfg.MaxSize > 0 && fh.Size > cfg.MaxSize {
		return nil, fmt.Errorf("upload: file %q exceeds max size of %d bytes", fh.Filename, cfg.MaxSize)
	}

	// Detect MIME type
	f, err := fh.Open()
	if err != nil {
		return nil, fmt.Errorf("upload: open: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	mime := http.DetectContentType(buf[:n])

	if len(cfg.AllowedTypes) > 0 && !isMIMEAllowed(mime, cfg.AllowedTypes) {
		return nil, fmt.Errorf("upload: file type %q is not allowed", mime)
	}

	return &UploadedFile{
		Header:   fh,
		Original: filepath.Base(fh.Filename),
		Size:     fh.Size,
		MIMEType: mime,
	}, nil
}

// Store saves the uploaded file to the given destination directory.
// Returns the full path to the saved file.
func (uf *UploadedFile) Store(dir string, filename ...string) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	name := uf.Original
	if len(filename) > 0 && filename[0] != "" {
		name = filename[0]
	}

	// Sanitize: remove path separators
	name = filepath.Base(strings.ReplaceAll(name, "..", ""))

	dest := filepath.Join(dir, name)
	out, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("upload: create dest: %w", err)
	}
	defer out.Close()

	src, err := uf.Header.Open()
	if err != nil {
		return "", fmt.Errorf("upload: open src: %w", err)
	}
	defer src.Close()

	if _, err := io.Copy(out, src); err != nil {
		return "", fmt.Errorf("upload: write: %w", err)
	}
	return dest, nil
}

// Open returns a ReadCloser for the uploaded file content.
func (uf *UploadedFile) Open() (multipart.File, error) { return uf.Header.Open() }

// Ext returns the lowercase file extension including the dot (e.g. ".jpg").
func (uf *UploadedFile) Ext() string {
	return strings.ToLower(filepath.Ext(uf.Original))
}

// IsImage reports whether the MIME type indicates an image.
func (uf *UploadedFile) IsImage() bool {
	return strings.HasPrefix(uf.MIMEType, "image/")
}

func isMIMEAllowed(mime string, allowed []string) bool {
	mime = strings.ToLower(mime)
	for _, a := range allowed {
		if strings.EqualFold(a, mime) || strings.HasPrefix(mime, strings.TrimSuffix(a, "*")) {
			return true
		}
	}
	return false
}
