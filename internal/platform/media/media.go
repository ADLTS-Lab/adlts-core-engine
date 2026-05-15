// Package media provides a platform-level media upload engine for ADLTS.
// It handles file validation, UUID-named storage, path-traversal protection,
// atomic replace (save-new → delete-old), and static file serving.
//
// Storage layout (mirrors Python MediaEngine):
//
//	<UPLOADS_DIR>/
//	├── candidates/<uuid>.jpg
//	├── experts/<uuid>.png
//	├── institutes/<uuid>.webp
//	└── transport-authorities/<uuid>.jpg
//
// UPLOADS_DIR defaults to "../uploads" (one level above project root),
// which survives Docker volume mounts as /uploads.
package media

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// ── Configuration ─────────────────────────────────────────────────────────────

var (
	allowedMIME = map[string]string{
		"image/jpeg": "jpg",
		"image/png":  "png",
		"image/webp": "webp",
	}
	// MaxFileSize is set from config at app startup.
	MaxFileSize int64 = 5 * 1024 * 1024 // 5 MB default
	// UploadsDir is the base directory; set from config at app startup.
	UploadsDir = "../uploads"
)

// ── Errors ────────────────────────────────────────────────────────────────────

type ErrInvalidMIME struct{ Got string }

func (e ErrInvalidMIME) Error() string {
	return fmt.Sprintf("unsupported file type '%s'; allowed: jpeg, png, webp", e.Got)
}

type ErrFileTooLarge struct{ MaxMB int64 }

func (e ErrFileTooLarge) Error() string {
	return fmt.Sprintf("file exceeds maximum allowed size of %d MB", e.MaxMB)
}

// ── Core engine ───────────────────────────────────────────────────────────────

// Save validates and persists a file under <UploadsDir>/<category>/<uuid>.<ext>.
// Returns the relative path (e.g. "candidates/abc123.jpg") stored in the DB.
func Save(category string, header *multipart.FileHeader) (string, error) {
	if err := validateSize(header.Size); err != nil {
		return "", err
	}
	contentType, err := detectMIME(header)
	if err != nil {
		return "", err
	}
	ext, ok := allowedMIME[contentType]
	if !ok {
		return "", ErrInvalidMIME{Got: contentType}
	}

	relPath := fmt.Sprintf("%s/%s.%s", category, uuid.New().String(), ext)
	absPath, err := safePath(relPath)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return "", fmt.Errorf("create upload dir: %w", err)
	}

	src, err := header.Open()
	if err != nil {
		return "", fmt.Errorf("open upload: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(absPath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	return relPath, nil
}

// Delete removes a file by its relative path. Returns nil if file not found.
func Delete(relPath string) error {
	if relPath == "" {
		return nil
	}
	absPath, err := safePath(relPath)
	if err != nil {
		return err
	}
	if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}

// Replace atomically swaps oldPath for the new upload.
// New file is saved first; old is deleted only on success.
func Replace(category string, header *multipart.FileHeader, oldPath string) (string, error) {
	newPath, err := Save(category, header)
	if err != nil {
		return "", err
	}
	// Best-effort delete of old file; don't fail if it's already gone.
	_ = Delete(oldPath)
	return newPath, nil
}

// ServeHandler returns an http.HandlerFunc that streams files from UploadsDir.
// Mount at /uploads/* so the wildcard captures the relative path.
func ServeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Strip leading /uploads/ prefix
		relPath := strings.TrimPrefix(r.URL.Path, "/uploads/")
		if relPath == "" || strings.Contains(relPath, "..") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		absPath, err := safePath(relPath)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		f, err := os.Open(absPath)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		defer f.Close()

		// Derive Content-Type from extension
		ext := filepath.Ext(absPath)
		ct := mime.TypeByExtension(ext)
		if ct == "" {
			ct = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		io.Copy(w, f) //nolint:errcheck
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// safePath resolves relPath under UploadsDir and guards against path traversal.
func safePath(relPath string) (string, error) {
	base, err := filepath.Abs(UploadsDir)
	if err != nil {
		return "", fmt.Errorf("resolve uploads dir: %w", err)
	}
	full, err := filepath.Abs(filepath.Join(base, relPath))
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if !strings.HasPrefix(full, base+string(os.PathSeparator)) && full != base {
		return "", fmt.Errorf("invalid path: traversal detected")
	}
	return full, nil
}

func validateSize(size int64) error {
	if size > MaxFileSize {
		return ErrFileTooLarge{MaxMB: MaxFileSize / (1024 * 1024)}
	}
	return nil
}

// detectMIME reads Content-Type from the multipart header, with fallback sniff.
func detectMIME(header *multipart.FileHeader) (string, error) {
	ct := header.Header.Get("Content-Type")
	if ct != "" {
		mt, _, _ := mime.ParseMediaType(ct)
		return mt, nil
	}
	// Sniff from first 512 bytes
	f, err := header.Open()
	if err != nil {
		return "", fmt.Errorf("open for sniff: %w", err)
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	return http.DetectContentType(buf[:n]), nil
}
