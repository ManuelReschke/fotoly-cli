package cli

import (
	"strings"
	"testing"

	"github.com/ManuelReschke/fotoly-cli/internal/brand"
)

func TestAlbumsLS(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv := mediaServer(t, nil)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)

	a.Root().SetArgs([]string{"albums", "ls"})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Cats") || !strings.Contains(out, "42") {
		t.Fatalf("stdout=%q", out)
	}
}
