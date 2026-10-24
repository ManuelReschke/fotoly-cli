package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ManuelReschke/fotoly-cli/internal/brand"
	"github.com/ManuelReschke/fotoly-cli/internal/client"
)

const (
	exportUUID1 = "11111111-1111-1111-1111-111111111111"
	exportUUID2 = "22222222-2222-2222-2222-222222222222"
)

type exportAPI struct {
	t         *testing.T
	post      string
	gets      []string
	images    []client.ExportImage
	bodies    map[string][]byte
	posts     int
	getN      int
	accountN  int
	downloads map[string]int
}

func (e *exportAPI) server(t *testing.T) *httptest.Server {
	t.Helper()
	e.t = t
	if e.downloads == nil {
		e.downloads = map[string]int{}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/user/export", e.handlePost)
	mux.HandleFunc("GET /api/v1/user/export", e.handleGet)
	mux.HandleFunc("GET /api/v1/user/export/account", e.handleAccount)
	mux.HandleFunc("GET /api/v1/user/export/files/{uuid}", e.handleFile)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func (e *exportAPI) checkKey(r *http.Request) {
	if r.Header.Get("X-API-Key") != "pxl_test" {
		e.t.Errorf("key=%q path=%s", r.Header.Get("X-API-Key"), r.URL.Path)
	}
}

func (e *exportAPI) handlePost(w http.ResponseWriter, r *http.Request) {
	e.checkKey(r)
	e.posts++
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, e.post)
}

func (e *exportAPI) handleGet(w http.ResponseWriter, r *http.Request) {
	e.checkKey(r)
	w.Header().Set("Content-Type", "application/json")
	if e.getN >= len(e.gets) {
		e.t.Errorf("unexpected GET /user/export (%d)", e.getN+1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"not_found","message":"missing export"}`)
		return
	}
	body := e.gets[e.getN]
	e.getN++
	_, _ = io.WriteString(w, body)
}

func (e *exportAPI) handleAccount(w http.ResponseWriter, r *http.Request) {
	e.checkKey(r)
	e.accountN++
	w.Header().Set("Content-Type", "application/json")
	doc := struct {
		ExportedAt string               `json:"exported_at"`
		Images     []client.ExportImage `json:"images"`
	}{ExportedAt: "2026-10-24T12:00:00Z", Images: e.images}
	_ = json.NewEncoder(w).Encode(doc)
}

func (e *exportAPI) handleFile(w http.ResponseWriter, r *http.Request) {
	e.checkKey(r)
	uuid := r.PathValue("uuid")
	e.downloads[uuid]++
	body, ok := e.bodies[uuid]
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(body)
}

func exportStatusJSON(status, errText string, created *bool) string {
	payload := map[string]any{
		"id":             1,
		"status":         status,
		"original_bytes": 0,
		"includes_zip":   false,
		"expires_at":     nil,
		"error":          errText,
	}
	if created != nil {
		payload["created"] = *created
	}
	b, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func exportImage(uuid, name string, body []byte) client.ExportImage {
	return client.ExportImage{
		UUID:         uuid,
		FileName:     name,
		FileSize:     int64(len(body)),
		SHA256:       sha256Hex(body),
		DownloadPath: "/user/export/files/" + uuid,
	}
}

func TestExportPendingThenReady(t *testing.T) {
	body := []byte("cat")
	created := false
	api := &exportAPI{
		post: exportStatusJSON("pending", "", &created),
		gets: []string{
			exportStatusJSON("pending", "", nil),
			exportStatusJSON("ready", "", nil),
		},
		images: []client.ExportImage{exportImage(exportUUID1, "photos/cat.jpg", body)},
		bodies: map[string][]byte{exportUUID1: body},
	}
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	var slept []time.Duration
	a.Sleep = func(d time.Duration) { slept = append(slept, d) }
	srv := api.server(t)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	dir := t.TempDir()
	a.Root().SetArgs([]string{"export", "--dir", dir})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if api.posts != 1 || api.getN != 2 || api.accountN != 1 || api.downloads[exportUUID1] != 1 {
		t.Fatalf("posts=%d gets=%d account=%d downloads=%v", api.posts, api.getN, api.accountN, api.downloads)
	}
	if len(slept) != 2 || slept[0] != 2*time.Second {
		t.Fatalf("slept=%v", slept)
	}
	account, err := os.ReadFile(filepath.Join(dir, "account.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc client.ExportAccount
	if err := json.Unmarshal(account, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.ExportedAt != "2026-10-24T12:00:00Z" || len(doc.Images) != 1 {
		t.Fatalf("account=%s", account)
	}
	got, err := os.ReadFile(filepath.Join(dir, "files", "cat.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "cat" {
		t.Fatalf("file=%q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "files", "photos")); !os.IsNotExist(err) {
		t.Fatalf("nested directory was created: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "saved "+filepath.Join("files", "cat.jpg")+" (1/1)\n") {
		t.Fatalf("stdout=%q", out)
	}
	if !strings.Contains(out, "export ready: "+dir+" (1 files)\n") {
		t.Fatalf("stdout=%q", out)
	}
	if strings.Contains(out, "skip ") || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", out, stderr.String())
	}
}

func TestExportResumeSkipsMatchingFile(t *testing.T) {
	body := []byte("cat")
	img := exportImage(exportUUID1, "cat.jpg", body)
	img.SHA256 = strings.ToUpper(img.SHA256)
	api := &exportAPI{
		post:   exportStatusJSON("ready", "", nil),
		images: []client.ExportImage{img},
		bodies: map[string][]byte{exportUUID1: body},
	}
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	a.Sleep = func(time.Duration) { t.Fatal("ready export must not sleep") }
	srv := api.server(t)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "files"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "files", "cat.jpg"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	a.Root().SetArgs([]string{"export", "--dir", dir})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if api.getN != 0 || api.downloads[exportUUID1] != 0 {
		t.Fatalf("gets=%d downloads=%v", api.getN, api.downloads)
	}
	out := stdout.String()
	if !strings.Contains(out, "skip "+filepath.Join("files", "cat.jpg")+"\n") {
		t.Fatalf("stdout=%q", out)
	}
	if strings.Contains(out, "saved ") {
		t.Fatalf("stdout=%q", out)
	}
	if !strings.Contains(out, "export ready: "+dir+" (1 files)\n") {
		t.Fatalf("stdout=%q", out)
	}
}

func TestExportSHA256MismatchRedownloads(t *testing.T) {
	body := []byte("abc")
	stale := []byte("xxx")
	if len(body) != len(stale) {
		t.Fatal("fixture sizes must match")
	}
	api := &exportAPI{
		post:   exportStatusJSON("ready", "", nil),
		images: []client.ExportImage{exportImage(exportUUID1, "cat.jpg", body)},
		bodies: map[string][]byte{exportUUID1: body},
	}
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := api.server(t)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "files"), 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "files", "cat.jpg")
	if err := os.WriteFile(dest, stale, 0o644); err != nil {
		t.Fatal(err)
	}
	a.Root().SetArgs([]string{"export", "--dir", dir})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if api.downloads[exportUUID1] != 1 {
		t.Fatalf("downloads=%v", api.downloads)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatalf("file=%q", got)
	}
	if !strings.Contains(stdout.String(), "saved ") || strings.Contains(stdout.String(), "skip ") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestExportFailedStatus(t *testing.T) {
	api := &exportAPI{
		post: exportStatusJSON("pending", "", nil),
		gets: []string{exportStatusJSON("failed", "disk full", nil)},
		images: []client.ExportImage{
			exportImage(exportUUID1, "cat.jpg", []byte("cat")),
		},
		bodies: map[string][]byte{exportUUID1: []byte("cat")},
	}
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := api.server(t)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	dir := t.TempDir()
	a.Root().SetArgs([]string{"export", "--dir", dir})
	code := Run(a)
	if code != 1 {
		t.Fatalf("code=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "disk full") {
		t.Fatalf("stderr=%q", stderr.String())
	}
	if api.accountN != 0 || api.downloads[exportUUID1] != 0 {
		t.Fatalf("account=%d downloads=%v", api.accountN, api.downloads)
	}
	if _, err := os.Stat(filepath.Join(dir, "account.json")); !os.IsNotExist(err) {
		t.Fatalf("account.json err=%v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestExportDuplicateFileNames(t *testing.T) {
	one := []byte("one")
	two := []byte("two!")
	api := &exportAPI{
		post: exportStatusJSON("ready", "", nil),
		images: []client.ExportImage{
			exportImage(exportUUID1, "cat.jpg", one),
			exportImage(exportUUID2, "album/cat.jpg", two),
		},
		bodies: map[string][]byte{exportUUID1: one, exportUUID2: two},
	}
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := api.server(t)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	dir := t.TempDir()
	a.Root().SetArgs([]string{"export", "--dir", dir})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	firstName := "cat__" + exportUUID1[:8] + ".jpg"
	secondName := "cat__" + exportUUID2[:8] + ".jpg"
	first, err := os.ReadFile(filepath.Join(dir, "files", firstName))
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(dir, "files", secondName))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != "one" || string(second) != "two!" {
		t.Fatalf("first=%q second=%q", first, second)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "files"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("files=%d", len(entries))
	}
	out := stdout.String()
	if !strings.Contains(out, "saved "+filepath.Join("files", firstName)+" (1/2)\n") {
		t.Fatalf("stdout=%q", out)
	}
	if !strings.Contains(out, "saved "+filepath.Join("files", secondName)+" (2/2)\n") {
		t.Fatalf("stdout=%q", out)
	}
	if !strings.Contains(out, "(2 files)") {
		t.Fatalf("stdout=%q", out)
	}
}

func TestExportJSONOmitsProgress(t *testing.T) {
	body := []byte("cat")
	api := &exportAPI{
		post:   exportStatusJSON("ready", "", nil),
		images: []client.ExportImage{exportImage(exportUUID1, "cat.jpg", body)},
		bodies: map[string][]byte{exportUUID1: body},
	}
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := api.server(t)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	dir := t.TempDir()
	a.Root().SetArgs([]string{"export", "--dir", dir, "--json"})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "saved ") || strings.Contains(stdout.String(), "skip ") || strings.Contains(stdout.String(), "export ready:") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	var got exportResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout=%q err=%v", stdout.String(), err)
	}
	if got.Dir != dir || got.Files != 1 || got.Skipped != 0 || got.Status != "ready" {
		t.Fatalf("got=%+v", got)
	}
	if strings.Count(stdout.String(), "\n") != 1 {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestExportJSONSkippedCount(t *testing.T) {
	body := []byte("cat")
	api := &exportAPI{
		post:   exportStatusJSON("ready", "", nil),
		images: []client.ExportImage{exportImage(exportUUID1, "cat.jpg", body)},
		bodies: map[string][]byte{exportUUID1: body},
	}
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := api.server(t)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "files"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "files", "cat.jpg"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	a.Root().SetArgs([]string{"--json", "export", "--dir", dir})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var got exportResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Files != 1 || got.Skipped != 1 || got.Status != "ready" || got.Dir != dir {
		t.Fatalf("got=%+v stdout=%s", got, stdout.String())
	}
}

func TestExportVerifyMismatchLeavesFile(t *testing.T) {
	bad := []byte("nope")
	img := exportImage(exportUUID1, "cat.jpg", []byte("cat"))
	api := &exportAPI{
		post:   exportStatusJSON("ready", "", nil),
		images: []client.ExportImage{img},
		bodies: map[string][]byte{exportUUID1: bad},
	}
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := api.server(t)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	dir := t.TempDir()
	a.Root().SetArgs([]string{"export", "--dir", dir})
	code := Run(a)
	if code != 1 {
		t.Fatalf("code=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "sha256") && !strings.Contains(stderr.String(), "file_size") {
		t.Fatalf("stderr=%q", stderr.String())
	}
	got, err := os.ReadFile(filepath.Join(dir, "files", "cat.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(bad) {
		t.Fatalf("bad file was removed or replaced: %q", got)
	}
	if strings.Contains(stdout.String(), "export ready:") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestExportEmptySHA256SkipsOnSize(t *testing.T) {
	body := []byte("cat")
	other := []byte("dog")
	img := exportImage(exportUUID1, "cat.jpg", body)
	img.SHA256 = ""
	img.FileSize = int64(len(other))
	api := &exportAPI{
		post:   exportStatusJSON("ready", "", nil),
		images: []client.ExportImage{img},
		bodies: map[string][]byte{exportUUID1: body},
	}
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := api.server(t)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "files"), 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "files", "cat.jpg")
	if err := os.WriteFile(dest, other, 0o644); err != nil {
		t.Fatal(err)
	}
	a.Root().SetArgs([]string{"export", "--dir", dir})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if api.downloads[exportUUID1] != 0 {
		t.Fatalf("downloads=%v", api.downloads)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(other) {
		t.Fatalf("file=%q", got)
	}
	if !strings.Contains(stdout.String(), "skip ") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestExportDefaultDirFlag(t *testing.T) {
	fotoly, _, _ := newTestApp(t, brand.Fotoly)
	pixelfox, _, _ := newTestApp(t, brand.PixelFox)
	if got := commandFlagDefault(t, fotoly, "export", "dir"); got != "./fotoly-export" {
		t.Fatalf("fotoly dir=%q", got)
	}
	if got := commandFlagDefault(t, pixelfox, "export", "dir"); got != "./pixelfox-export" {
		t.Fatalf("pixelfox dir=%q", got)
	}
}

func TestExportSafeNames(t *testing.T) {
	names := exportFileNames([]client.ExportImage{
		{UUID: exportUUID1, FileName: "nested/cat.jpg"},
		{UUID: "", FileName: ""},
		{UUID: exportUUID2, FileName: "cat.jpg"},
		{UUID: "44444444-4444-4444-4444-444444444444", FileName: `..\cat.jpg`},
		{UUID: "55555555-5555-5555-5555-555555555555", FileName: ".."},
	})
	want := []string{
		"cat__" + exportUUID1[:8] + ".jpg",
		"file",
		"cat__" + exportUUID2[:8] + ".jpg",
		"cat__44444444.jpg",
		"55555555-5555-5555-5555-555555555555",
	}
	if strings.Join(names, "\n") != strings.Join(want, "\n") {
		t.Fatalf("names=%q want=%q", names, want)
	}
}

func commandFlagDefault(t *testing.T, a *App, name, flag string) string {
	t.Helper()
	for _, cmd := range a.Root().Commands() {
		if cmd.Name() != name {
			continue
		}
		got, err := cmd.Flags().GetString(flag)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	t.Fatalf("command %s not found", name)
	return ""
}
