package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/ManuelReschke/fotoly-cli/internal/brand"
)

func TestAuthAndHealthCommands(t *testing.T) {
	const session = `{"token":"pxls_new","token_type":"Bearer","expires_at":"2027-01-01T00:00:00Z","user":{"username":"Ada"}}`
	for _, tc := range []struct {
		name                  string
		args                  []string
		method, path, body    string
		authorization, apiKey string
		stdin, response, want string
		status                int
	}{
		{name: "ping", args: []string{"ping"}, method: "GET", path: "/ping", response: `{"ping":"pong"}`, want: `"ping":"pong"`},
		{name: "providers", args: []string{"auth", "providers"}, method: "GET", path: "/auth/providers", response: `{"providers":[{"id":"google","name":"Google"}]}`, want: `"id":"google"`},
		{name: "start", args: []string{"auth", "start", "google"}, method: "GET", path: "/auth/google/start", status: 307, want: "/browser/oauth"},
		{name: "login", args: []string{"auth", "login", "--email", "ada@example.com", "--password-stdin"}, method: "POST", path: "/auth/login", stdin: " abc123 \n", body: `{"email":"ada@example.com","password":" abc123 "}`, response: session, want: `"token":"pxls_new"`},
		{name: "register", args: []string{"auth", "register", "--username", "Ada", "--email", "ada@example.com", "--password-stdin"}, method: "POST", path: "/auth/register", stdin: "abc123\n", body: `{"username":"Ada","email":"ada@example.com","password":"abc123"}`, response: `{"email":"ada@example.com","message":"Check your email"}`, status: 201, want: "Check your email"},
		{name: "token", args: []string{"auth", "token", "--code-stdin"}, method: "POST", path: "/auth/token", stdin: "oauth-code\n", body: `{"code":"oauth-code"}`, response: session, want: `"token":"pxls_new"`},
		{name: "logout", args: []string{"auth", "logout", "--api-key", "pxl_override"}, method: "POST", path: "/auth/logout", authorization: "Bearer pxls_env", status: 204, want: `"status":"revoked"`},
		{name: "handoff with session", args: []string{"auth", "web-handoff"}, method: "POST", path: "/auth/web-handoff", authorization: "Bearer pxls_env", response: `{"url":"https://fotoly.eu/auth/app?code=one-time"}`, want: "code=one-time"},
		{name: "handoff with key", args: []string{"auth", "web-handoff", "--next", "/pricing", "--api-key", "pxl_override"}, method: "POST", path: "/auth/web-handoff", body: `{"next":"/pricing"}`, apiKey: "pxl_override", response: `{"url":"https://fotoly.eu/auth/app?code=one-time"}`, want: "code=one-time"},
		{name: "profile with session", args: []string{"whoami"}, method: "GET", path: "/user/profile", authorization: "Bearer pxls_env", response: `{"username":"Ada","stats":{"images":{"storage_remaining_bytes":42}},"preferences":{"upload_ai_generated_by_default":true}}`, want: `"storage_remaining_bytes":42`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.method || r.URL.Path != "/api/v1"+tc.path {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.RequestURI())
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if r.Header.Get("Authorization") != tc.authorization || r.Header.Get("X-API-Key") != tc.apiKey {
					t.Error("unexpected auth headers")
				}
				body, err := io.ReadAll(r.Body)
				if err != nil || string(body) != tc.body {
					t.Errorf("unexpected body %q (%v)", body, err)
				}
				if tc.status == http.StatusTemporaryRedirect {
					if r.URL.Query().Get("redirect_uri") != "pixelfox://auth/callback" {
						t.Errorf("unexpected callback %q", r.URL.RawQuery)
					}
					w.Header().Set("Location", "/browser/oauth")
				}
				if tc.status != 0 {
					w.WriteHeader(tc.status)
				}
				_, _ = io.WriteString(w, tc.response)
			}))
			defer srv.Close()

			a, stdout, stderr := newTestApp(t, brand.PixelFox)
			a.LookupEnv = func(key string) string {
				switch key {
				case "PIXELFOX_API_KEY":
					return "pxl_env"
				case "PIXELFOX_SESSION_TOKEN":
					return "pxls_env"
				}
				return ""
			}
			attachServer(a, srv)
			a.Root().SetIn(strings.NewReader(tc.stdin))
			a.Root().SetArgs(append(tc.args, "--json"))
			if code := Run(a); code != 0 || !strings.Contains(stdout.String(), tc.want) || !json.Valid(stdout.Bytes()) {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
}

func TestUploadCheck(t *testing.T) {
	for _, origin := range []string{"", "https://fotoly.eu"} {
		t.Run(origin, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if origin == "" && r.Method != http.MethodHead ||
					origin != "" && (r.Method != http.MethodOptions || r.Header.Get("Origin") != origin ||
						r.Header.Get("Access-Control-Request-Method") != "POST") {
					t.Errorf("unexpected probe %s %v", r.Method, r.Header)
				}
				if r.URL.Path != "/api/v1/upload" || r.Header.Get("X-API-Key") != "" {
					t.Error("unexpected path or auth header")
				}
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()
			a, stdout, stderr := newTestApp(t, brand.Fotoly)
			attachServer(a, srv)
			a.Root().SetArgs([]string{"upload-check", "--origin", origin, "--api-key", "pxl_test", "--json"})
			if code := Run(a); code != 0 || !strings.Contains(stdout.String(), `"status_code":204`) {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
}

func TestAPIMetadataFlags(t *testing.T) {
	for _, tc := range []struct {
		args                      []string
		body, query, resp, output string
	}{
		{args: []string{"images", "ls", "--sort", "asc", "--ai-generated=false"}, query: "is_ai_generated=false&limit=25&sort=asc", resp: `{"items":[{"is_ai_generated":true,"tags":["cat"]}]}`, output: `"tags":["cat"]`},
		{args: []string{"images", "edit", "img", "--ai-generated=false"}, body: `{"is_ai_generated":false}`, resp: `{"is_ai_generated":false}`, output: `"is_ai_generated":false`},
		{args: []string{"albums", "create", "--title", "AI", "--ai-generated"}, body: `{"title":"AI","is_ai_generated":true}`, resp: `{"is_ai_generated":true}`, output: `"is_ai_generated":true`},
		{args: []string{"albums", "edit", "7", "--ai-generated=false"}, body: `{"is_ai_generated":false}`, resp: `{"cover_stable_url":"https://fotoly.eu/cover"}`, output: `"cover_stable_url":"https://fotoly.eu/cover"`},
		{args: []string{"albums", "members", "add", "7", "--email", "ada@example.com", "--role", "contributor"}, body: `{"email":"ada@example.com","role":"contributor"}`, resp: `{"role":"contributor"}`, output: `"role":"contributor"`},
		{args: []string{"albums", "categories", "create", "--name", "travel", "--scope", "private"}, body: `{"name":"travel","scope":"private"}`, resp: `{"scope":"private"}`, output: `"scope":"private"`},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			a, stdout, stderr := newTestApp(t, brand.Fotoly)
			srv, got := scriptedAPI(t, http.StatusOK, tc.resp)
			attachServer(a, srv)
			a.Root().SetArgs(append(tc.args, "--api-key", "pxl_test", "--json"))
			if code := Run(a); code != 0 || !strings.Contains(stdout.String(), tc.output) {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if got.Body != tc.body || got.Query.Encode() != tc.query {
				t.Fatalf("body=%s query=%v", got.Body, got.Query)
			}
		})
	}
}

func TestUploadCustomOptions(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	h := newUploadHarness(t)
	attachServer(a, h.srv)
	path := writeTempUpload(t, "cat.jpg", "hello")
	a.Root().SetArgs([]string{"upload", path, "--processing", "custom", "--derivative", "webp/small", "--derivative", "avif/medium",
		"--ai-generated", "--nsfw=false", "--tag", "cat", "--tag", "travel", "--upload-token-form", "--no-wait", "--api-key", "pxl_test"})
	if code := Run(a); code != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.uploadFormToken != "tok" {
		t.Fatal("upload token missing from multipart form")
	}
	body := h.sessionBody
	if body.IsAIGenerated == nil || !*body.IsAIGenerated || body.IsNSFW == nil || *body.IsNSFW ||
		!reflect.DeepEqual(body.Tags, []string{"cat", "travel"}) {
		t.Fatalf("session metadata %+v", body)
	}
	if body.Processing == nil || body.Processing.Profile != "custom" || len(body.Processing.Derivatives) != 2 ||
		body.Processing.Derivatives[0].Family != "webp" || body.Processing.Derivatives[1].Size != "medium" {
		t.Fatalf("processing %+v", body.Processing)
	}
}
