package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ManuelReschke/fotoly-cli/internal/brand"
	"github.com/ManuelReschke/fotoly-cli/internal/client"
)

type uploadHarness struct {
	srv           *httptest.Server
	mu            sync.Mutex
	sessionBody   client.UploadSessionRequest
	uploadCount   int
	statusCount   int
	statusPending bool
}

func newUploadHarness(t *testing.T) *uploadHarness {
	t.Helper()
	h := &uploadHarness{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/upload/sessions", func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		defer h.mu.Unlock()
		_ = json.NewDecoder(r.Body).Decode(&h.sessionBody)
		resp := map[string]any{
			"upload_url": h.srv.URL + "/api/v1/upload",
			"token":      "tok",
			"pool_id":    1,
			"expires_at": 9999999999,
			"max_bytes":  10_000_000,
		}
		if h.sessionBody.AlbumID != nil && *h.sessionBody.AlbumID == 42 {
			resp["album_id"] = 42
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("POST /api/v1/upload", func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.uploadCount++
		h.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"image_uuid":"img-1","view_url":"/i/share1","duplicate":false}`)
	})
	mux.HandleFunc("GET /api/v1/images/{uuid}/status", func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.statusCount++
		pending := h.statusPending
		h.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if pending {
			_, _ = io.WriteString(w, `{"complete":false,"failed":false}`)
			return
		}
		_, _ = io.WriteString(w, `{"complete":true,"failed":false}`)
	})
	mux.HandleFunc("GET /api/v1/images/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"image_uuid":"img-1","view_url":"/i/share1"}`)
	})
	h.srv = httptest.NewServer(mux)
	t.Cleanup(h.srv.Close)
	return h
}

func (h *uploadHarness) uploads() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.uploadCount
}

func (h *uploadHarness) statuses() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.statusCount
}

func (h *uploadHarness) fileSize() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sessionBody.FileSize
}

func writeTempUpload(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUploadHappyPath(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	h := newUploadHarness(t)
	attachServer(a, h.srv)
	saveTestAPIKey(t, a, h.srv.URL)
	path := writeTempUpload(t, "cat.jpg", "hello")

	a.Root().SetArgs([]string{"upload", path})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "http://127.0.0.1") || !strings.Contains(out, "/i/share1") {
		t.Fatalf("stdout=%q", out)
	}
	if h.fileSize() != 5 {
		t.Fatalf("session file_size=%d", h.fileSize())
	}
}

func TestUploadJSON(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	h := newUploadHarness(t)
	attachServer(a, h.srv)
	saveTestAPIKey(t, a, h.srv.URL)
	path := writeTempUpload(t, "cat.jpg", "hello")

	a.Root().SetArgs([]string{"upload", "--json", path})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	var results []struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
		t.Fatalf("stdout not JSON array: %v %q", err, stdout.String())
	}
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestUploadNoWaitSkipsStatus(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	h := newUploadHarness(t)
	attachServer(a, h.srv)
	saveTestAPIKey(t, a, h.srv.URL)
	path := writeTempUpload(t, "cat.jpg", "hello")

	a.Root().SetArgs([]string{"upload", "--no-wait", "--json", path})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if h.statuses() != 0 {
		t.Fatalf("status calls=%d", h.statuses())
	}
	if !strings.Contains(stdout.String(), "img-1") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestUploadMissingFileContinues(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	h := newUploadHarness(t)
	attachServer(a, h.srv)
	saveTestAPIKey(t, a, h.srv.URL)
	path := writeTempUpload(t, "cat.jpg", "hello")

	a.Root().SetArgs([]string{"upload", "missing.jpg", path})
	code := Run(a)
	if code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if h.uploads() != 1 {
		t.Fatalf("upload POSTs=%d", h.uploads())
	}
}

func TestUploadProcessingInvalid(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	path := writeTempUpload(t, "cat.jpg", "hello")
	a.Root().SetArgs([]string{"upload", "--processing", "custom", path})
	code := Run(a)
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "processing") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestUploadCopyAndNoCopyConflict(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	path := writeTempUpload(t, "cat.jpg", "hello")
	a.Root().SetArgs([]string{"upload", "--copy", "--no-copy", path})
	code := Run(a)
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "copy") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestUploadClipboardSingleTTY(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	h := newUploadHarness(t)
	attachServer(a, h.srv)
	saveTestAPIKey(t, a, h.srv.URL)
	path := writeTempUpload(t, "cat.jpg", "hello")
	var copied string
	a.IsTTY = func() bool { return true }
	a.Clipboard = func(s string) error {
		copied = s
		return nil
	}

	a.Root().SetArgs([]string{"upload", path})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(copied, "/i/share1") {
		t.Fatalf("clipboard=%q", copied)
	}
}

func TestUploadAlbumNotBoundWarns(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	h := newUploadHarness(t)
	attachServer(a, h.srv)
	saveTestAPIKey(t, a, h.srv.URL)
	path := writeTempUpload(t, "cat.jpg", "hello")

	a.Root().SetArgs([]string{"upload", "--album", "99", path})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "not bound") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestUploadZeroFiles(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	a.Root().SetArgs([]string{"upload"})
	code := Run(a)
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "at least one file") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestUploadProcessingTimeout(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	h := newUploadHarness(t)
	h.statusPending = true
	attachServer(a, h.srv)
	saveTestAPIKey(t, a, h.srv.URL)
	path := writeTempUpload(t, "cat.jpg", "hello")
	a.Sleep = func(time.Duration) {}

	a.Root().SetArgs([]string{"upload", path})
	code := Run(a)
	if code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "timed out") {
		t.Fatalf("stderr=%q", stderr.String())
	}
	if h.statuses() != 45 {
		t.Fatalf("GetImageStatus calls=%d", h.statuses())
	}
}
