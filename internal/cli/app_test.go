package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ManuelReschke/fotoly-cli/internal/brand"
)

func newTestApp(t *testing.T, br brand.Brand) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	dir := t.TempDir()
	a := New(br)
	a.Version = "1.2.3"
	a.Commit = "abc"
	a.Stdout = stdout
	a.Stderr = stderr
	a.UserConfigDir = func() (string, error) { return dir, nil }
	a.LookupEnv = func(string) string { return "" }
	a.IsTTY = func() bool { return false }
	a.Sleep = func(time.Duration) {}
	a.Prompter = stubPrompter{}
	return a, stdout, stderr
}

func TestVersion(t *testing.T) {
	a, stdout, _ := newTestApp(t, brand.Fotoly)
	cmd := a.Root()
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "fotoly") || !strings.Contains(out, "1.2.3") || !strings.Contains(out, "abc") {
		t.Fatalf("out=%q", out)
	}
}

func TestRootUseIsBinary(t *testing.T) {
	a, _, _ := newTestApp(t, brand.PixelFox)
	if a.Root().Use != "pixelfox" {
		t.Fatalf("Use=%q", a.Root().Use)
	}
}

func TestHelpDoesNotRequireAuth(t *testing.T) {
	a, _, _ := newTestApp(t, brand.Fotoly)
	cmd := a.Root()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownCommandReturnsUsageExit(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	a.Root().SetArgs([]string{"nope"})
	code := Run(a)
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}
