package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/ManuelReschke/fotoly-cli/internal/brand"
	"github.com/ManuelReschke/fotoly-cli/internal/config"
)

func TestWhoamiNonTTYNoKey(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	a.Root().SetArgs([]string{"whoami"})
	code := Run(a)
	if code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "No API key. Run 'fotoly setup'.") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestWhoamiWithSavedKey(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := profileServer(t, http.StatusOK, "pixelpete", "premium")
	attachServer(a, srv)
	dir := testConfigDir(t, a)
	if err := config.Save(config.FilePath(brand.Fotoly, dir), config.File{BaseURL: srv.URL, APIKey: "pxl_test"}); err != nil {
		t.Fatal(err)
	}
	a.Root().SetArgs([]string{"whoami"})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Username") || !strings.Contains(stdout.String(), "pixelpete") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestWhoamiJSON(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := profileServer(t, http.StatusOK, "pixelpete", "premium")
	attachServer(a, srv)
	dir := testConfigDir(t, a)
	if err := config.Save(config.FilePath(brand.Fotoly, dir), config.File{BaseURL: srv.URL, APIKey: "pxl_test"}); err != nil {
		t.Fatal(err)
	}
	a.Root().SetArgs([]string{"whoami", "--json"})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	var acc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &acc); err != nil {
		t.Fatalf("stdout not JSON: %v %q", err, stdout.String())
	}
	if acc["username"] != "pixelpete" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestWhoamiTTYNoKeyRunsSetupThenProfile(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := profileServer(t, http.StatusOK, "pixelpete", "premium")
	attachServer(a, srv)
	a.IsTTY = func() bool { return true }
	a.Prompter = stubPrompter{key: "pxl_from_ui", url: srv.URL}
	a.Root().SetArgs([]string{"whoami"})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "pixelpete") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	vals, err := config.Load(brand.Fotoly, config.Source{UserConfigDir: testConfigDir(t, a), LookupEnv: func(string) string { return "" }})
	if err != nil {
		t.Fatal(err)
	}
	if vals.APIKey != "pxl_from_ui" {
		t.Fatalf("APIKey=%q", vals.APIKey)
	}
}

func TestWhoamiAliasMe(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := profileServer(t, http.StatusOK, "pixelpete", "premium")
	attachServer(a, srv)
	dir := testConfigDir(t, a)
	if err := config.Save(config.FilePath(brand.Fotoly, dir), config.File{BaseURL: srv.URL, APIKey: "pxl_test"}); err != nil {
		t.Fatal(err)
	}
	a.Root().SetArgs([]string{"me"})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "pixelpete") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestWhoamiJSONUnauthorized(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := profileServer(t, http.StatusUnauthorized, "", "")
	attachServer(a, srv)
	dir := testConfigDir(t, a)
	if err := config.Save(config.FilePath(brand.Fotoly, dir), config.File{BaseURL: srv.URL, APIKey: "pxl_bad"}); err != nil {
		t.Fatal(err)
	}
	a.Root().SetArgs([]string{"whoami", "--json"})
	code := Run(a)
	if code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout=%q", stdout.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("stderr not JSON: %v %q", err, stderr.String())
	}
	msg, _ := payload["message"].(string)
	if !strings.Contains(msg, "Invalid API key") {
		t.Fatalf("stderr=%q", stderr.String())
	}
	if _, ok := payload["error"]; !ok {
		t.Fatalf("missing error field: %q", stderr.String())
	}
}

func TestWhoamiUnauthorized(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	srv := profileServer(t, http.StatusUnauthorized, "", "")
	attachServer(a, srv)
	dir := testConfigDir(t, a)
	if err := config.Save(config.FilePath(brand.Fotoly, dir), config.File{BaseURL: srv.URL, APIKey: "pxl_bad"}); err != nil {
		t.Fatal(err)
	}
	a.Root().SetArgs([]string{"whoami"})
	code := Run(a)
	if code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Invalid API key") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}
