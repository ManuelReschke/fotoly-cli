package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/ManuelReschke/fotoly-cli/internal/brand"
	"github.com/ManuelReschke/fotoly-cli/internal/config"
)

const testImageUUID = "12345678-aaaa-bbbb-cccc-ddddeeeeffff"

type deleteLog struct {
	mu    sync.Mutex
	uuids []string
}

func (d *deleteLog) add(uuid string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.uuids = append(d.uuids, uuid)
}

func (d *deleteLog) called() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.uuids) > 0
}

func mediaServer(t *testing.T, deleted *deleteLog) *httptest.Server {
	t.Helper()
	if deleted == nil {
		deleted = &deleteLog{}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/albums", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"albums":[{"id":42,"title":"Cats","description":"","is_public":true,"is_nsfw":false,"share_link":"s","view_url":"/a/s","image_count":2,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]}`)
	})
	mux.HandleFunc("GET /api/v1/images", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"items":[{"image_uuid":"12345678-aaaa-bbbb-cccc-ddddeeeeffff","title":"","description":"","file_name":"cat.jpg","file_size":100,"file_type":"image/jpeg","width":1,"height":1,"is_public":true,"is_nsfw":false,"share_link":"s","view_url":"/i/s","stable_url":"https://x/s","view_count":0,"download_count":0,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}],"has_more":true,"next_cursor":"cur1"}`)
	})
	mux.HandleFunc("GET /api/v1/images/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"image_uuid":"u","view_url":"/i/s","url":"https://cdn.example/orig.jpg","is_nsfw":false,"available_variants":["original","webp"],"variants":{"original":{"original":{"url":"https://cdn.example/orig.jpg"}},"webp":{"medium":{"url":"https://cdn.example/m.webp"}}},"processing":{"profile":"default","keep_original":true}}`)
	})
	mux.HandleFunc("DELETE /api/v1/images/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		deleted.add(r.PathValue("uuid"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"image_uuid":"u","status":"accepted","message":"queued"}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func saveTestAPIKey(t *testing.T, a *App, baseURL string) {
	t.Helper()
	if err := config.Save(config.FilePath(brand.Fotoly, testConfigDir(t, a)), config.File{BaseURL: baseURL, APIKey: "pxl_test"}); err != nil {
		t.Fatal(err)
	}
}

func TestImagesLSTruncatesUUID(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := mediaServer(t, nil)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)

	a.Root().SetArgs([]string{"images", "ls"})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "12345678") {
		t.Fatalf("stdout=%q", out)
	}
	if strings.Contains(out, testImageUUID) {
		t.Fatalf("human output must truncate uuid: %q", out)
	}

	a2, stdout2, stderr2 := newTestApp(t, brand.Fotoly)
	attachServer(a2, srv)
	saveTestAPIKey(t, a2, srv.URL)
	a2.Root().SetArgs([]string{"images", "ls", "--json"})
	code = Run(a2)
	if code != 0 {
		t.Fatalf("json code=%d stderr=%s", code, stderr2.String())
	}
	if !strings.Contains(stdout2.String(), testImageUUID) {
		t.Fatalf("json stdout=%q", stdout2.String())
	}
}

func TestImagesLSPublicPrivateConflict(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	a.Root().SetArgs([]string{"images", "ls", "--public", "--private"})
	code := Run(a)
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--public and --private are mutually exclusive") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestImagesGet(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := mediaServer(t, nil)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)

	a.Root().SetArgs([]string{"images", "get", "u"})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	want := srv.URL + "/i/s"
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("stdout=%q want share URL %q", stdout.String(), want)
	}
}

func TestImagesGetJSONIncludesVariants(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := mediaServer(t, nil)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)

	a.Root().SetArgs([]string{"images", "get", "u", "--json"})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	var img map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &img); err != nil {
		t.Fatalf("stdout not JSON: %v %q", err, stdout.String())
	}
	variants, ok := img["variants"].(map[string]any)
	if !ok {
		t.Fatalf("stdout=%q", stdout.String())
	}
	orig, _ := variants["original"].(map[string]any)
	origSize, _ := orig["original"].(map[string]any)
	if origSize["url"] != "https://cdn.example/orig.jpg" {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if _, ok := img["processing"].(map[string]any); !ok {
		t.Fatalf("missing processing: %q", stdout.String())
	}
}

func TestImagesLSUsageJSONStaysText(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	a.Root().SetArgs([]string{"images", "ls", "--json", "--public", "--private"})
	code := Run(a)
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	s := strings.TrimSpace(stderr.String())
	if strings.HasPrefix(s, "{") {
		t.Fatalf("usage error should stay text, stderr=%q", stderr.String())
	}
	if !strings.Contains(s, "--public and --private are mutually exclusive") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestImagesDeleteRequiresYesNonTTY(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	deleted := &deleteLog{}
	srv := mediaServer(t, deleted)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)

	a.Root().SetArgs([]string{"images", "delete", "u"})
	code := Run(a)
	if code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if deleted.called() {
		t.Fatal("DELETE must not be called without --yes on non-TTY")
	}
	if !strings.Contains(stderr.String(), "use --yes to delete") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestImagesDeleteYes(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	deleted := &deleteLog{}
	srv := mediaServer(t, deleted)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)

	a.Root().SetArgs([]string{"images", "delete", "--yes", "u"})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !deleted.called() {
		t.Fatal("DELETE not called")
	}
	out := stdout.String() + stderr.String()
	if !strings.Contains(out, "accepted") {
		t.Fatalf("stdout+stderr=%q", out)
	}
}

func TestImagesDeleteTTYDeclined(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	deleted := &deleteLog{}
	srv := mediaServer(t, deleted)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.IsTTY = func() bool { return true }
	a.Prompter = stubPrompter{confirm: false}

	a.Root().SetArgs([]string{"images", "delete", "u"})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if deleted.called() {
		t.Fatal("DELETE must not be called when confirm is declined")
	}
	if !strings.Contains(stdout.String(), "Aborted.") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestImagesLSJSONHasMoreCursorOnStderr(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := mediaServer(t, nil)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)

	a.Root().SetArgs([]string{"images", "ls", "--json"})
	code := Run(a)
	if code != 0 {
		t.Fatalf("json code=%d stderr=%s", code, stderr.String())
	}
	var col map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &col); err != nil {
		t.Fatalf("stdout not JSON: %v %q", err, stdout.String())
	}
	if col["next_cursor"] != "cur1" {
		t.Fatalf("json stdout=%q", stdout.String())
	}

	a2, _, stderr2 := newTestApp(t, brand.Fotoly)
	attachServer(a2, srv)
	saveTestAPIKey(t, a2, srv.URL)
	a2.Root().SetArgs([]string{"images", "ls"})
	code = Run(a2)
	if code != 0 {
		t.Fatalf("human code=%d stderr=%s", code, stderr2.String())
	}
	if !strings.Contains(stderr2.String(), "next cursor: cur1") {
		t.Fatalf("stderr=%q", stderr2.String())
	}
}
