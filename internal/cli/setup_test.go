package cli

import (
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
	"github.com/ManuelReschke/fotoly-cli/internal/config"
)

type stubPrompter struct {
	key, url string
	err      error
	confirm  bool
}

func (s stubPrompter) PromptSetup(brand.Brand, string) (string, string, error) {
	return s.key, s.url, s.err
}
func (s stubPrompter) ConfirmDelete(int) (bool, error) { return s.confirm, nil }

func profileServer(t *testing.T, status int, username, plan string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = io.WriteString(w, `{"error":"unauthorized","message":"Invalid API key"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"username": username,
			"plan":     plan,
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func attachServer(a *App, srv *httptest.Server) {
	c := srv.Client()
	c.Timeout = 5 * time.Second
	a.HTTPClient = c
	prev := a.LookupEnv
	a.LookupEnv = func(k string) string {
		if k == a.Brand.EnvPrefix+"_BASE_URL" {
			return srv.URL
		}
		if prev != nil {
			return prev(k)
		}
		return ""
	}
}

func testConfigDir(t *testing.T, a *App) string {
	t.Helper()
	dir, err := a.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSetupNonTTYNoKeyExits1(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	a.Root().SetArgs([]string{"setup"})
	code := Run(a)
	if code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "No API key. Use --api-key or run setup from a terminal.") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestSetupAPIKeyWritesConfig(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := profileServer(t, http.StatusOK, "pixelpete", "free")
	attachServer(a, srv)
	a.Root().SetArgs([]string{"setup", "--api-key", "pxl_test"})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	dir := testConfigDir(t, a)
	vals, err := config.Load(brand.Fotoly, config.Source{UserConfigDir: dir, LookupEnv: func(string) string { return "" }})
	if err != nil {
		t.Fatal(err)
	}
	if vals.APIKey != "pxl_test" {
		t.Fatalf("APIKey=%q", vals.APIKey)
	}
	want := "Logged in as pixelpete (free) on " + srv.URL
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("stdout=%q want %q", stdout.String(), want)
	}
	out := stdout.String() + stderr.String()
	if strings.Contains(out, "pxl_test") {
		t.Fatalf("must not print API key: %q", out)
	}
}

func TestSetupInvalidKey(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	srv := profileServer(t, http.StatusUnauthorized, "", "")
	attachServer(a, srv)
	a.Root().SetArgs([]string{"setup", "--api-key", "pxl_bad"})
	code := Run(a)
	if code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Invalid API key. Run 'fotoly setup'.") {
		t.Fatalf("stderr=%q", stderr.String())
	}
	path := config.FilePath(brand.Fotoly, testConfigDir(t, a))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("config should not be saved on 401: %v", err)
	}
}

func TestSetupReset(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	dir := testConfigDir(t, a)
	path := config.FilePath(brand.Fotoly, dir)
	if err := config.Save(path, config.File{BaseURL: brand.Fotoly.DefaultURL, APIKey: "pxl_old"}); err != nil {
		t.Fatal(err)
	}
	a.Root().SetArgs([]string{"setup", "--reset"})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Config removed.") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file still exists: %v", err)
	}
}

func TestSetupResetInvalidTOML(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	dir := testConfigDir(t, a)
	path := config.FilePath(brand.Fotoly, dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("this is not toml {{{"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.Root().SetArgs([]string{"setup", "--reset"})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Config removed.") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file still exists: %v", err)
	}
}

func TestSetupWarnsNonPxlPrefix(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	srv := profileServer(t, http.StatusOK, "pixelpete", "free")
	attachServer(a, srv)
	a.Root().SetArgs([]string{"setup", "--api-key", "notaprefix"})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "pxl_") {
		t.Fatalf("stderr=%q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Warning: API keys usually start with pxl_") {
		t.Fatalf("stderr=%q", stderr.String())
	}
	vals, err := config.Load(brand.Fotoly, config.Source{UserConfigDir: testConfigDir(t, a), LookupEnv: func(string) string { return "" }})
	if err != nil {
		t.Fatal(err)
	}
	if vals.APIKey != "notaprefix" {
		t.Fatalf("APIKey=%q", vals.APIKey)
	}
}

func TestSetupTTYUsesPrompter(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := profileServer(t, http.StatusOK, "pixelpete", "free")
	c := srv.Client()
	c.Timeout = 5 * time.Second
	a.HTTPClient = c
	a.IsTTY = func() bool { return true }
	a.Prompter = stubPrompter{key: "pxl_from_ui", url: srv.URL}
	a.Root().SetArgs([]string{"setup"})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	vals, err := config.Load(brand.Fotoly, config.Source{UserConfigDir: testConfigDir(t, a), LookupEnv: func(string) string { return "" }})
	if err != nil {
		t.Fatal(err)
	}
	if vals.APIKey != "pxl_from_ui" {
		t.Fatalf("APIKey=%q", vals.APIKey)
	}
	if vals.BaseURL != srv.URL {
		t.Fatalf("BaseURL=%q want %q", vals.BaseURL, srv.URL)
	}
	want := "Logged in as pixelpete (free) on " + srv.URL
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("stdout=%q want %q", stdout.String(), want)
	}
}
