# fotoly-cli v1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a hybrid Cobra + Charm CLI as two binaries (`fotoly`, `pixelfox`) that store an API key on first run and can upload, list, show, and delete images against the PixelFox/Fotoly CMS API v1.

**Architecture:** One Go module. Thin `cmd/fotoly` and `cmd/pixelfox` inject a `brand.Brand`. Shared packages: `internal/brand`, `internal/config`, `internal/client`, `internal/ui`, `internal/cli`. Commands never call HTTP directly; they go through `client.Client`. Tests use temp dirs and `httptest` only.

**Tech Stack:** Go 1.26, Cobra, Charm huh + lipgloss, pelletier TOML v2, `net/http`, atotto clipboard. Spec: `docs/superpowers/specs/2026-09-08-fotoly-cli-design.md`. API shapes: PixelFox `public/docs/v1/openapi.yml`.

## Global Constraints

- Go version: 1.26
- Module path: `github.com/ManuelReschke/fotoly-cli`
- English UI only
- Auth: user API key via `X-API-Key` (not app-session `pxls_` tokens)
- Upload POST to `upload_url` uses `Authorization: Bearer {session token}` and must not send the user API key
- Config: `filepath.Join(os.UserConfigDir(), brand.ConfigApp, "config.toml")`, dir `0700`, file `0600`
- Precedence: flag > env > file
- Env: `{PREFIX}_API_KEY`, `{PREFIX}_BASE_URL`, `{PREFIX}_CONFIG`
- Processing flag values: `default` (CLI default) or `original_only`
- Status poll: 1s interval, 45 attempts
- Accent: Fotoly `#E85D04`, PixelFox `#F97316`
- Do not print or log the full API key
- No live HTTP to fotoly.eu / pixelfox.cc in tests
- No generated OpenAPI client
- `NO_COLOR` set or non-TTY → plain text
- Exit codes: 0 success, 1 runtime/API/validation, 2 usage

---

## File map

Create:

- `go.mod`, `go.sum`
- `.gitignore`
- `Makefile`
- `cmd/fotoly/main.go`
- `cmd/pixelfox/main.go`
- `internal/brand/brand.go`
- `internal/brand/brand_test.go`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/client/types.go`
- `internal/client/error.go`
- `internal/client/client.go`
- `internal/client/client_test.go`
- `internal/client/share.go`
- `internal/client/share_test.go`
- `internal/ui/ui.go`
- `internal/ui/ui_test.go`
- `internal/cli/app.go`
- `internal/cli/root.go`
- `internal/cli/setup.go`
- `internal/cli/whoami.go`
- `internal/cli/albums.go`
- `internal/cli/images.go`
- `internal/cli/upload.go`
- `internal/cli/prompter.go`
- `internal/cli/app_test.go`
- `internal/cli/setup_test.go`
- `internal/cli/whoami_test.go`
- `internal/cli/albums_test.go`
- `internal/cli/images_test.go`
- `internal/cli/upload_test.go`
- `README.md` (replace the one-liner)

Do not modify `LICENSE`.

---

## Shared types (verbatim — every task uses these names)

```go
package brand

type Brand struct {
    Name       string // "Fotoly" / "PixelFox"
    Binary     string // "fotoly" / "pixelfox"
    DefaultURL string // "https://fotoly.eu" / "https://pixelfox.cc"
    ConfigApp  string // "fotoly" / "pixelfox"
    EnvPrefix  string // "FOTOLY" / "PIXELFOX"
    UAPrefix   string // "fotoly-cli" / "pixelfox-cli"
    Accent     string // "#E85D04" / "#F97316"
}

func (b Brand) APIRoot(baseURL string) string
func (b Brand) UserAgentString(version string) string

var Fotoly Brand
var PixelFox Brand
```

```go
package config

type File struct {
    BaseURL string `toml:"base_url"`
    APIKey  string `toml:"api_key"`
}

type Values struct {
    Path    string
    BaseURL string
    APIKey  string
}

type Source struct {
    UserConfigDir string
    LookupEnv     func(key string) string
    FlagConfig    string
    FlagAPIKey    string
}

func FilePath(b brand.Brand, userConfigDir string) string
func Load(b brand.Brand, src Source) (Values, error)
func Save(path string, file File) error
func Reset(path string) error
```

```go
package client

type Client struct {
    BaseURL    string
    APIKey     string
    UserAgent  string
    HTTP       *http.Client
}

func New(baseURL, apiKey, userAgent string, httpClient *http.Client) *Client
func (c *Client) GetProfile(ctx context.Context) (*UserAccount, error)
func (c *Client) CreateUploadSession(ctx context.Context, req UploadSessionRequest) (*UploadSessionResponse, error)
func (c *Client) UploadFile(ctx context.Context, uploadURL, token, filename string, r io.Reader, size int64, progress func(sent, total int64)) (*StorageUploadResponse, error)
func (c *Client) GetImageStatus(ctx context.Context, uuid string) (*ImageStatus, error)
func (c *Client) GetImage(ctx context.Context, uuid string) (*ImageResource, error)
func (c *Client) ListImages(ctx context.Context, q ImageListQuery) (*ImageCollection, error)
func (c *Client) DeleteImage(ctx context.Context, uuid string) (*ImageDeletionAccepted, error)
func (c *Client) ListAlbums(ctx context.Context) (*AlbumCollection, error)
func ResolveShareURL(baseURL string, candidates ...string) (string, error)

type APIError struct {
    Status  int
    Code    string
    Message string
}
func (e *APIError) Error() string
func IsUnauthorized(err error) bool
```

JSON field names on client structs MUST match PixelFox OpenAPI (`image_uuid`, `view_url`, `file_name`, `has_more`, `next_cursor`, `upload_url`, `max_bytes`, `error`, `message`). Do not invent names.

```go
package cli

type App struct {
    Brand         brand.Brand
    Version       string
    Commit        string
    Stdout        io.Writer
    Stderr        io.Writer
    LookupEnv     func(string) string
    UserConfigDir func() (string, error)
    HTTPClient    *http.Client
    IsTTY         func() bool
    Prompter      Prompter
    Clipboard     func(string) error
    Sleep         func(time.Duration)
}

type Prompter interface {
    PromptSetup(br brand.Brand, defaultURL string) (apiKey, baseURL string, err error)
    ConfirmDelete(n int) (bool, error)
}

func New(b brand.Brand) *App
func (a *App) Root() *cobra.Command
func Execute(b brand.Brand) int
```

---

### Task 1: Module, gitignore, brand

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `internal/brand/brand.go`
- Create: `internal/brand/brand_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `brand.Brand`, `brand.Fotoly`, `brand.PixelFox`, `Brand.APIRoot`, `Brand.UserAgentString`

- [ ] **Step 1: Write the failing tests**

```go
package brand

import "testing"

func TestBrandsAreIsolated(t *testing.T) {
    if Fotoly.DefaultURL == PixelFox.DefaultURL {
        t.Fatal("brands must not share DefaultURL")
    }
    if Fotoly.ConfigApp == PixelFox.ConfigApp {
        t.Fatal("brands must not share ConfigApp")
    }
    if Fotoly.EnvPrefix == PixelFox.EnvPrefix {
        t.Fatal("brands must not share EnvPrefix")
    }
}

func TestFotolyDefaults(t *testing.T) {
    if Fotoly.Name != "Fotoly" || Fotoly.Binary != "fotoly" {
        t.Fatalf("fotoly identity: %+v", Fotoly)
    }
    if Fotoly.DefaultURL != "https://fotoly.eu" {
        t.Fatalf("DefaultURL=%q", Fotoly.DefaultURL)
    }
    if Fotoly.ConfigApp != "fotoly" || Fotoly.EnvPrefix != "FOTOLY" {
        t.Fatalf("config/env %+v", Fotoly)
    }
    if Fotoly.Accent != "#E85D04" {
        t.Fatalf("Accent=%q", Fotoly.Accent)
    }
}

func TestPixelFoxDefaults(t *testing.T) {
    if PixelFox.Name != "PixelFox" || PixelFox.Binary != "pixelfox" {
        t.Fatalf("pixelfox identity: %+v", PixelFox)
    }
    if PixelFox.DefaultURL != "https://pixelfox.cc" {
        t.Fatalf("DefaultURL=%q", PixelFox.DefaultURL)
    }
    if PixelFox.ConfigApp != "pixelfox" || PixelFox.EnvPrefix != "PIXELFOX" {
        t.Fatalf("config/env %+v", PixelFox)
    }
    if PixelFox.Accent != "#F97316" {
        t.Fatalf("Accent=%q", PixelFox.Accent)
    }
}

func TestUserAgentString(t *testing.T) {
    if got := Fotoly.UserAgentString("1.0.0"); got != "fotoly-cli/1.0.0" {
        t.Fatalf("got %q", got)
    }
    if got := PixelFox.UserAgentString(""); got != "pixelfox-cli/dev" {
        t.Fatalf("got %q", got)
    }
}

func TestAPIRoot(t *testing.T) {
    if got := Fotoly.APIRoot("https://fotoly.eu"); got != "https://fotoly.eu/api/v1" {
        t.Fatalf("got %q", got)
    }
    if got := Fotoly.APIRoot("https://fotoly.eu/"); got != "https://fotoly.eu/api/v1" {
        t.Fatalf("got %q", got)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/brand/ -count=1`

Expected: FAIL (`go.mod` missing and/or `undefined: Fotoly`)

- [ ] **Step 3: Write minimal implementation**

`go.mod`:

```
module github.com/ManuelReschke/fotoly-cli

go 1.26
```

`.gitignore`:

```
/bin/
/dist/
*.exe
.idea/
```

`internal/brand/brand.go`:

```go
package brand

import "strings"

type Brand struct {
    Name       string
    Binary     string
    DefaultURL string
    ConfigApp  string
    EnvPrefix  string
    UAPrefix   string
    Accent     string
}

func (b Brand) APIRoot(baseURL string) string {
    return strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/api/v1"
}

func (b Brand) UserAgentString(version string) string {
    if strings.TrimSpace(version) == "" {
        version = "dev"
    }
    return b.UAPrefix + "/" + version
}

var Fotoly = Brand{
    Name:       "Fotoly",
    Binary:     "fotoly",
    DefaultURL: "https://fotoly.eu",
    ConfigApp:  "fotoly",
    EnvPrefix:  "FOTOLY",
    UAPrefix:   "fotoly-cli",
    Accent:     "#E85D04",
}

var PixelFox = Brand{
    Name:       "PixelFox",
    Binary:     "pixelfox",
    DefaultURL: "https://pixelfox.cc",
    ConfigApp:  "pixelfox",
    EnvPrefix:  "PIXELFOX",
    UAPrefix:   "pixelfox-cli",
    Accent:     "#F97316",
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/brand/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go.mod .gitignore internal/brand/brand.go internal/brand/brand_test.go
git commit -m "feat: add module and isolated Fotoly/PixelFox brands"
```

---

### Task 2: Config load, save, reset, env overlay

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Interfaces:**
- Consumes: `brand.Brand`, `brand.Fotoly`, `brand.PixelFox`
- Produces: `config.File`, `config.Values`, `config.Source`, `FilePath`, `Load`, `Save`, `Reset`

- [ ] **Step 1: Write the failing tests**

```go
package config

import (
    "os"
    "path/filepath"
    "runtime"
    "testing"

    "github.com/ManuelReschke/fotoly-cli/internal/brand"
)

func TestFilePath(t *testing.T) {
    got := FilePath(brand.Fotoly, "/tmp/cfg")
    want := filepath.Join("/tmp/cfg", "fotoly", "config.toml")
    if got != want {
        t.Fatalf("got %q want %q", got, want)
    }
    if FilePath(brand.Fotoly, "/tmp/cfg") == FilePath(brand.PixelFox, "/tmp/cfg") {
        t.Fatal("brands must not share config path")
    }
}

func TestLoadMissingFileUsesBrandDefault(t *testing.T) {
    dir := t.TempDir()
    vals, err := Load(brand.Fotoly, Source{UserConfigDir: dir, LookupEnv: func(string) string { return "" }})
    if err != nil {
        t.Fatal(err)
    }
    if vals.APIKey != "" {
        t.Fatalf("APIKey=%q", vals.APIKey)
    }
    if vals.BaseURL != brand.Fotoly.DefaultURL {
        t.Fatalf("BaseURL=%q", vals.BaseURL)
    }
    if vals.Path != FilePath(brand.Fotoly, dir) {
        t.Fatalf("Path=%q", vals.Path)
    }
}

func TestSaveRoundTripAndMode(t *testing.T) {
    dir := t.TempDir()
    path := FilePath(brand.Fotoly, dir)
    if err := Save(path, File{BaseURL: "https://fotoly.eu", APIKey: "pxl_secret"}); err != nil {
        t.Fatal(err)
    }
    vals, err := Load(brand.Fotoly, Source{UserConfigDir: dir, LookupEnv: func(string) string { return "" }})
    if err != nil {
        t.Fatal(err)
    }
    if vals.APIKey != "pxl_secret" || vals.BaseURL != "https://fotoly.eu" {
        t.Fatalf("vals=%+v", vals)
    }
    if runtime.GOOS != "windows" {
        info, err := os.Stat(path)
        if err != nil {
            t.Fatal(err)
        }
        if info.Mode().Perm() != 0o600 {
            t.Fatalf("perm=%o", info.Mode().Perm())
        }
        dirInfo, err := os.Stat(filepath.Dir(path))
        if err != nil {
            t.Fatal(err)
        }
        if dirInfo.Mode().Perm() != 0o700 {
            t.Fatalf("dir perm=%o", dirInfo.Mode().Perm())
        }
    }
}

func TestEnvOverridesFile(t *testing.T) {
    dir := t.TempDir()
    path := FilePath(brand.Fotoly, dir)
    if err := Save(path, File{BaseURL: "https://fotoly.eu", APIKey: "pxl_file"}); err != nil {
        t.Fatal(err)
    }
    env := map[string]string{
        "FOTOLY_API_KEY":  "pxl_env",
        "FOTOLY_BASE_URL": "http://localhost:8080",
    }
    vals, err := Load(brand.Fotoly, Source{
        UserConfigDir: dir,
        LookupEnv:     func(k string) string { return env[k] },
    })
    if err != nil {
        t.Fatal(err)
    }
    if vals.APIKey != "pxl_env" {
        t.Fatalf("APIKey=%q", vals.APIKey)
    }
    if vals.BaseURL != "http://localhost:8080" {
        t.Fatalf("BaseURL=%q", vals.BaseURL)
    }
}

func TestFlagOverridesEnv(t *testing.T) {
    env := map[string]string{"FOTOLY_API_KEY": "pxl_env"}
    vals, err := Load(brand.Fotoly, Source{
        UserConfigDir: t.TempDir(),
        LookupEnv:     func(k string) string { return env[k] },
        FlagAPIKey:    "pxl_flag",
    })
    if err != nil {
        t.Fatal(err)
    }
    if vals.APIKey != "pxl_flag" {
        t.Fatalf("APIKey=%q", vals.APIKey)
    }
}

func TestFotolyEnvDoesNotLeakIntoPixelFox(t *testing.T) {
    env := map[string]string{"FOTOLY_API_KEY": "pxl_fotoly"}
    vals, err := Load(brand.PixelFox, Source{
        UserConfigDir: t.TempDir(),
        LookupEnv:     func(k string) string { return env[k] },
    })
    if err != nil {
        t.Fatal(err)
    }
    if vals.APIKey != "" {
        t.Fatalf("PixelFox picked up Fotoly key: %q", vals.APIKey)
    }
    if vals.BaseURL != brand.PixelFox.DefaultURL {
        t.Fatalf("BaseURL=%q", vals.BaseURL)
    }
}

func TestFlagConfigPath(t *testing.T) {
    dir := t.TempDir()
    custom := filepath.Join(dir, "custom.toml")
    if err := Save(custom, File{BaseURL: "https://fotoly.eu", APIKey: "pxl_custom"}); err != nil {
        t.Fatal(err)
    }
    vals, err := Load(brand.Fotoly, Source{
        UserConfigDir: dir,
        LookupEnv:     func(string) string { return "" },
        FlagConfig:    custom,
    })
    if err != nil {
        t.Fatal(err)
    }
    if vals.APIKey != "pxl_custom" || vals.Path != custom {
        t.Fatalf("vals=%+v", vals)
    }
}

func TestEnvConfigPath(t *testing.T) {
    dir := t.TempDir()
    custom := filepath.Join(dir, "from-env.toml")
    if err := Save(custom, File{BaseURL: "https://fotoly.eu", APIKey: "pxl_envpath"}); err != nil {
        t.Fatal(err)
    }
    env := map[string]string{"FOTOLY_CONFIG": custom}
    vals, err := Load(brand.Fotoly, Source{
        UserConfigDir: dir,
        LookupEnv:     func(k string) string { return env[k] },
    })
    if err != nil {
        t.Fatal(err)
    }
    if vals.APIKey != "pxl_envpath" {
        t.Fatalf("APIKey=%q", vals.APIKey)
    }
}

func TestResetMissingIsSuccess(t *testing.T) {
    if err := Reset(filepath.Join(t.TempDir(), "missing.toml")); err != nil {
        t.Fatal(err)
    }
}

func TestResetDeletesFile(t *testing.T) {
    dir := t.TempDir()
    path := FilePath(brand.Fotoly, dir)
    if err := Save(path, File{APIKey: "pxl_x", BaseURL: "https://fotoly.eu"}); err != nil {
        t.Fatal(err)
    }
    if err := Reset(path); err != nil {
        t.Fatal(err)
    }
    if _, err := os.Stat(path); !os.IsNotExist(err) {
        t.Fatalf("file still exists: %v", err)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go get github.com/pelletier/go-toml/v2 && go test ./internal/config/ -count=1`

Expected: FAIL (`undefined: FilePath` / package not found)

- [ ] **Step 3: Write minimal implementation**

`internal/config/config.go`:

- `FilePath` = `filepath.Join(userConfigDir, b.ConfigApp, "config.toml")`
- `Save`: `os.MkdirAll(dir, 0o700)`, `os.WriteFile(path, tomlBytes, 0o600)`, then `os.Chmod(path, 0o600)`
- `Load`: resolve path as FlagConfig else `LookupEnv(prefix+"_CONFIG")` else `FilePath`. If file missing, APIKey empty and BaseURL = brand default. If file present, unmarshal TOML. Then overlay: APIKey = FlagAPIKey else env `prefix+"_API_KEY"` else file. BaseURL = env `prefix+"_BASE_URL"` else file else brand default. Trim spaces. Empty BaseURL after overlay still falls back to `b.DefaultURL`.
- `Reset`: `os.Remove`; `os.IsNotExist` → nil
- `LookupEnv` nil → treat as empty

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/config/
git commit -m "feat: add TOML config with 0600 perms and env overlay"
```

---

### Task 3: HTTP client — errors, profile, share URL

**Files:**
- Create: `internal/client/types.go`
- Create: `internal/client/error.go`
- Create: `internal/client/client.go`
- Create: `internal/client/share.go`
- Create: `internal/client/client_test.go`
- Create: `internal/client/share_test.go`

**Interfaces:**
- Consumes: `brand.Brand.APIRoot` (callers pass already-resolved BaseURL; client appends `/api/v1` itself via `brand`-independent join: BaseURL is the site origin, client requests `{trim(BaseURL)}/api/v1/...`)
- Produces: `client.Client`, `New`, `GetProfile`, `APIError`, `IsUnauthorized`, `ResolveShareURL`, types `UserAccount` (fields: `Username`, `Email`, `Plan`, `Status`, nested `Stats.Images.Count`, `Stats.Images.StorageUsedBytes`, `Limits.MaxUploadBytes`, `Limits.StorageQuotaBytes`)

Client BaseURL is the **site origin** (`https://fotoly.eu`), not the API root. Request path helper: `strings.TrimRight(c.BaseURL, "/") + "/api/v1" + path` where path starts with `/`.

- [ ] **Step 1: Write the failing tests**

`share_test.go`:

```go
package client

import "testing"

func TestResolveShareURLRelative(t *testing.T) {
    got, err := ResolveShareURL("https://fotoly.eu", "/i/abc")
    if err != nil {
        t.Fatal(err)
    }
    if got != "https://fotoly.eu/i/abc" {
        t.Fatalf("got %q", got)
    }
}

func TestResolveShareURLAbsolute(t *testing.T) {
    got, err := ResolveShareURL("https://fotoly.eu", "https://cdn.example/i/abc")
    if err != nil {
        t.Fatal(err)
    }
    if got != "https://cdn.example/i/abc" {
        t.Fatalf("got %q", got)
    }
}

func TestResolveShareURLFirstNonEmpty(t *testing.T) {
    got, err := ResolveShareURL("https://fotoly.eu", "", "/i/from-view")
    if err != nil {
        t.Fatal(err)
    }
    if got != "https://fotoly.eu/i/from-view" {
        t.Fatalf("got %q", got)
    }
}

func TestResolveShareURLNone(t *testing.T) {
    _, err := ResolveShareURL("https://fotoly.eu")
    if err == nil {
        t.Fatal("expected error")
    }
}
```

`client_test.go` (profile + 401):

```go
package client

import (
    "context"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestGetProfileSuccessSendsAPIKey(t *testing.T) {
    var gotKey, gotUA, gotPath string
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        gotKey = r.Header.Get("X-API-Key")
        gotUA = r.Header.Get("User-Agent")
        gotPath = r.URL.Path
        if r.Header.Get("Authorization") != "" {
            t.Errorf("profile must not send Authorization, got %q", r.Header.Get("Authorization"))
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{
            "id": 1, "username": "pixelpete", "email": "pete@example.com",
            "status": "active", "plan": "premium", "created_at": "2026-01-01T00:00:00Z",
            "stats":  map[string]any{"images": map[string]any{"count": 3, "storage_used_bytes": 100}, "albums": map[string]any{"count": 1}},
            "limits": map[string]any{"max_upload_bytes": 50, "storage_quota_bytes": 200, "can_multi_upload": true, "image_upload_enabled": true, "direct_upload_enabled": true, "allowed_thumbnail_formats": []string{"original"}},
            "preferences": map[string]any{"upload_nsfw_by_default": false, "thumbnail_original": true, "thumbnail_webp": false, "thumbnail_avif": false},
        })
    }))
    defer srv.Close()

    c := New(srv.URL, "pxl_test", "fotoly-cli/dev", srv.Client())
    acc, err := c.GetProfile(context.Background())
    if err != nil {
        t.Fatal(err)
    }
    if acc.Username != "pixelpete" || acc.Plan != "premium" {
        t.Fatalf("account=%+v", acc)
    }
    if gotKey != "pxl_test" || !strings.Contains(gotUA, "fotoly-cli") || gotPath != "/api/v1/user/profile" {
        t.Fatalf("key=%q ua=%q path=%q", gotKey, gotUA, gotPath)
    }
}

func TestGetProfileUnauthorized(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusUnauthorized)
        _, _ = io.WriteString(w, `{"error":"unauthorized","message":"Invalid API key"}`)
    }))
    defer srv.Close()

    c := New(srv.URL, "bad", "fotoly-cli/dev", srv.Client())
    _, err := c.GetProfile(context.Background())
    if !IsUnauthorized(err) {
        t.Fatalf("err=%v", err)
    }
    apiErr, ok := err.(*APIError)
    if !ok || apiErr.Message != "Invalid API key" || apiErr.Status != 401 {
        t.Fatalf("apiErr=%v", err)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/ -count=1`

Expected: FAIL (`undefined: New`)

- [ ] **Step 3: Write minimal implementation**

`error.go`: `APIError.Error()` returns `Message` if set, else `Code`. `IsUnauthorized` uses `errors.As` and `Status == 401`.

`share.go`: walk candidates, skip empty; `url.Parse` the candidate; if not absolute, `base.ResolveReference`. Error if none.

`client.go`: `New` uses `http.DefaultClient` when httpClient is nil. `doJSON` sets Accept, X-API-Key, User-Agent. Decode error body into `{error, message}`. Timeout: if httpClient is DefaultClient, wrap a clone with 30s timeout inside `New` only when httpClient == nil (`&http.Client{Timeout: 30 * time.Second}`).

`types.go`: define `UserAccount` with json tags matching the test payload. Nested anonymous structs or named `UserAccountStats` / `UserAccountLimits` as in the spec.

`GetProfile`: `GET /user/profile`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/client/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/client/
git commit -m "feat: add API client profile fetch, errors, and share URL resolver"
```

---

### Task 4: Client — albums, images list/get/delete, status

**Files:**
- Modify: `internal/client/types.go`
- Modify: `internal/client/client.go`
- Modify: `internal/client/client_test.go`

**Interfaces:**
- Consumes: `Client.doJSON` from Task 3
- Produces: `ListAlbums`, `ListImages`, `GetImage`, `GetImageStatus`, `DeleteImage`, `ImageListQuery`, `ImageCollection`, `ImageSummary`, `ImageResource`, `ImageStatus`, `ImageDeletionAccepted`, `AlbumCollection`, `AlbumSummary`

```go
type ImageListQuery struct {
    Limit    int
    Cursor   string
    AlbumID  int64
    IsPublic *bool
    IsNSFW   *bool
    Tag      string
}

type ImageStatus struct {
    Complete bool    `json:"complete"`
    Failed   bool    `json:"failed"`
    ViewURL  *string `json:"view_url"`
}
```

- [ ] **Step 1: Write the failing tests**

Append to `client_test.go`:

- `TestListAlbums` — GET `/api/v1/albums`, decode `{"albums":[{"id":42,"title":"Cats","description":"","is_public":true,"is_nsfw":false,"share_link":"s","view_url":"/a/s","image_count":2,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]}`
- `TestListImagesQuery` — GET `/api/v1/images` with query `limit=10&cursor=abc&album_id=42&is_public=true&is_nsfw=false&tag=cat`. Assert those query values. Decode `items`, `has_more`, `next_cursor`.
- `TestGetImage` — GET `/api/v1/images/{uuid}`
- `TestGetImageStatus` — GET `/api/v1/images/{uuid}/status`
- `TestDeleteImage` — DELETE `/api/v1/images/{uuid}`, status 202, body `{"image_uuid":"u","status":"accepted","message":"queued"}`
- `TestGetImageNotFound` — 404 → `APIError.Status == 404`.

Omit unset query params: Limit 0 means server default (do not send `limit`). AlbumID 0 omit. nil bools omit. empty cursor/tag omit.

Canonical query-string test:

```go
func TestListImagesQuery(t *testing.T) {
    pub, nsfw := true, false
    var got url.Values
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/api/v1/images" {
            t.Fatalf("path=%s", r.URL.Path)
        }
        got = r.URL.Query()
        w.Header().Set("Content-Type", "application/json")
        _, _ = io.WriteString(w, `{"items":[{"image_uuid":"u1","title":"","description":"","file_name":"a.jpg","file_size":10,"file_type":"image/jpeg","width":1,"height":1,"is_public":true,"is_nsfw":false,"share_link":"s","view_url":"/i/s","stable_url":"https://x/s","view_count":0,"download_count":0,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}],"has_more":true,"next_cursor":"cur1"}`)
    }))
    defer srv.Close()
    c := New(srv.URL, "pxl_test", "fotoly-cli/dev", srv.Client())
    col, err := c.ListImages(context.Background(), ImageListQuery{Limit: 10, Cursor: "abc", AlbumID: 42, IsPublic: &pub, IsNSFW: &nsfw, Tag: "cat"})
    if err != nil {
        t.Fatal(err)
    }
    if !col.HasMore || col.NextCursor == nil || *col.NextCursor != "cur1" || len(col.Items) != 1 {
        t.Fatalf("col=%+v", col)
    }
    if got.Get("limit") != "10" || got.Get("cursor") != "abc" || got.Get("album_id") != "42" || got.Get("is_public") != "true" || got.Get("is_nsfw") != "false" || got.Get("tag") != "cat" {
        t.Fatalf("query=%v", got)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/ -count=1`

Expected: FAIL (`undefined: ListAlbums`)

- [ ] **Step 3: Write minimal implementation**

Implement the five methods. `ListImages` builds `url.Values`. `DeleteImage` treats 202 as success.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/client/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/client/
git commit -m "feat: add image and album API client methods"
```

---

### Task 5: Client — upload session and multipart upload

**Files:**
- Modify: `internal/client/types.go`
- Modify: `internal/client/client.go`
- Modify: `internal/client/client_test.go`

**Interfaces:**
- Consumes: `Client` from Task 3
- Produces: `CreateUploadSession`, `UploadFile`, `UploadSessionRequest`, `UploadSessionResponse`, `StorageUploadResponse`

```go
type UploadSessionRequest struct {
    FileSize   int64   `json:"file_size"`
    AlbumID    *int64  `json:"album_id,omitempty"`
    IsNSFW     *bool   `json:"is_nsfw,omitempty"`
    Processing *struct {
        Profile string `json:"profile"`
    } `json:"processing,omitempty"`
}

type UploadSessionResponse struct {
    UploadURL string `json:"upload_url"`
    Token     string `json:"token"`
    PoolID    int64  `json:"pool_id"`
    ExpiresAt int64  `json:"expires_at"`
    MaxBytes  int64  `json:"max_bytes"`
    AlbumID   *int64 `json:"album_id,omitempty"`
}

type StorageUploadResponse struct {
    ImageUUID *string `json:"image_uuid"`
    ViewURL   *string `json:"view_url"`
    URL       *string `json:"url"`
    Duplicate *bool   `json:"duplicate"`
}
```

- [ ] **Step 1: Write the failing tests**

`TestCreateUploadSession`:

- POST `/api/v1/upload/sessions` with JSON `file_size` and optional album/nsfw/processing
- Assert `X-API-Key` present
- Return `{upload_url, token, pool_id, expires_at, max_bytes}`

`TestUploadFileUsesBearerNotAPIKey`:

- Separate httptest server for the upload URL
- POST multipart field name `file`
- Assert `Authorization == "Bearer sess_token"`
- Assert `X-API-Key` is empty
- Progress callback called with sent > 0
- Response JSON `{image_uuid, view_url, duplicate}`

`TestCreateUploadSessionTooLarge`: 413 body `{"error":"quota_exceeded","message":"storage quota exceeded"}` → `APIError.Status == 413`

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/ -count=1`

Expected: FAIL (`undefined: CreateUploadSession`)

- [ ] **Step 3: Write minimal implementation**

`CreateUploadSession`: POST JSON.

`UploadFile`: `multipart.NewWriter`, create form file `"file"`, copy from `r` through a `io.TeeReader` or custom reader that invokes `progress`. POST to `uploadURL` as given (absolute). Header `Authorization: Bearer {token}` only. Do not attach X-API-Key. Use a client timeout of at least 10 minutes for this request: clone `c.HTTP` transport but set `Timeout: 10 * time.Minute` on a one-off `http.Client` (keep Transport). If `c.HTTP.Transport` is nil, use `http.DefaultTransport`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/client/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/client/
git commit -m "feat: add upload session and multipart upload client"
```

---

### Task 6: CLI app skeleton, version, persistent flags, two mains

**Files:**
- Create: `internal/ui/ui.go`
- Create: `internal/ui/ui_test.go`
- Create: `internal/cli/app.go`
- Create: `internal/cli/root.go`
- Create: `internal/cli/app_test.go`
- Create: `cmd/fotoly/main.go`
- Create: `cmd/pixelfox/main.go`
- Create: `Makefile`

**Interfaces:**
- Consumes: `brand.Brand`, `config.Source` (wired later)
- Produces: `cli.App`, `cli.New`, `App.Root`, `cli.Execute`, persistent flags `--json`, `--config`, `--api-key`, command `version`, `ui.Enabled`, `ui.Header`, `ui.Table`

`ui.Enabled(noColorEnv string, stdoutIsTTY bool) bool` is true only when `noColorEnv == ""` and `stdoutIsTTY`.

`App.Root()` Use = `a.Brand.Binary`. Short = `Official CLI for {Name} ({DefaultURL})`.

`version` prints `{Binary} {Version} ({Commit})` to stdout. Default Version/Commit `dev` / `unknown`.

`Execute` returns 0 on success, 2 if cobra usage error (`cobra.SilenceUsage` only for non-usage errors — use `SilenceUsage: true` on root and return 2 when `errors.Is(err, flag.ErrHelp)` is false and the error is cobra's `cmd.SilenceErrors`... Practical rule: `Execute` returns 2 when `err != nil` and stdout/stderr contain `Usage:`; otherwise 1 on error; 0 on nil. Simpler and required: inspect `cobra.Command.ExecuteC()`; if `err != nil` and `cmd.FlagError` / args error, exit 2. Implementation: set `SilenceErrors: true`. In `Execute`:

```go
root := app.Root()
if err := root.Execute(); err != nil {
    fmt.Fprintln(app.Stderr, err.Error())
    var apiErr *client.APIError
    if errors.As(err, &apiErr) || err.Error() != "" {
        if strings.Contains(strings.ToLower(err.Error()), "unknown command") || strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "usage") {
            return 2
        }
    }
    return 1
}
```

That is too fuzzy. Required implementation:

```go
func Execute(b brand.Brand) int {
    a := New(b)
    cmd := a.Root()
    cmd.SetOut(a.Stdout)
    cmd.SetErr(a.Stderr)
    if err := cmd.Execute(); err != nil {
        fmt.Fprintln(a.Stderr, err.Error())
        if errors.Is(err, ErrUsage) {
            return 2
        }
        return 1
    }
    return 0
}
```

Define `var ErrUsage = errors.New("usage")`. Commands that need exit 2 `return fmt.Errorf("%w: %s", ErrUsage, msg)`.

Mains:

```go
package main

import (
    "os"
    "github.com/ManuelReschke/fotoly-cli/internal/brand"
    "github.com/ManuelReschke/fotoly-cli/internal/cli"
)

var (
    version = "dev"
    commit  = "unknown"
)

func main() {
    os.Exit(run())
}

func run() int {
    a := cli.New(brand.Fotoly)
    a.Version = version
    a.Commit = commit
    return cli.Run(a)
}
```

Add `func Run(a *App) int` so tests and Execute share it. `Execute(b)` = `Run(New(b))`.

`cmd/pixelfox/main.go` is identical except `brand.PixelFox`.

- [ ] **Step 1: Write the failing tests**

`internal/ui/ui_test.go`:

```go
package ui

import "testing"

func TestEnabled(t *testing.T) {
    if !Enabled("", true) {
        t.Fatal("expected color on TTY")
    }
    if Enabled("1", true) {
        t.Fatal("NO_COLOR disables")
    }
    if Enabled("", false) {
        t.Fatal("non-TTY disables")
    }
}
```

`internal/cli/app_test.go`:

```go
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
    a := New(br)
    a.Version = "1.2.3"
    a.Commit = "abc"
    a.Stdout = stdout
    a.Stderr = stderr
    a.UserConfigDir = func() (string, error) { return t.TempDir(), nil }
    a.LookupEnv = func(string) string { return "" }
    a.IsTTY = func() bool { return false }
    a.Sleep = func(time.Duration) {}
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
```

`Run` must apply `SetArgs` already on `a.Root()` — cobra remembers the command object. `Run` should call `cmd := a.Root()` once. Problem: `Root()` builds a new command each call. **Required:** cache the root command on `App`:

```go
func (a *App) Root() *cobra.Command {
    if a.root == nil {
        a.root = a.buildRoot()
    }
    return a.root
}
```

Tests call `a.Root().SetArgs(...)` then `Run(a)` which uses the same cached command. `Run` must not rebuild.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go get github.com/spf13/cobra github.com/charmbracelet/lipgloss && go test ./internal/ui/ ./internal/cli/ -count=1`

Expected: FAIL (missing packages)

- [ ] **Step 3: Write minimal implementation**

`ui.go`: `Enabled`, `Header(accent, text string, colorOn bool) string` (lipgloss bold + foreground when colorOn), `Table(headers []string, rows [][]string) string` using lipgloss table or a simple tabwriter if lipgloss table API is awkward — tabwriter is acceptable for v1 if lipgloss is still used for Header.

`app.go`: `New` defaults Stdout/Stderr to os.Stdout/os.Stderr, LookupEnv to os.LookupEnv wrapped as `func(k string) string { v, _ := os.LookupEnv(k); return v }`, UserConfigDir to `os.UserConfigDir`, IsTTY to `term.IsTerminal(int(os.Stderr.Fd()))` via `golang.org/x/term`, HTTPClient nil (client.New will create one), Sleep `time.Sleep`, Clipboard no-op returning `errors.New("clipboard unavailable")` until Task 10 replaces default with atotto.

`root.go`: cobra root, persistent flags:

```go
cmd.PersistentFlags().Bool("json", false, "output JSON")
cmd.PersistentFlags().String("config", "", "config file path")
cmd.PersistentFlags().String("api-key", "", "API key (overrides config)")
```

Store on App during PersistentPreRun:

```go
a.flagJSON, _ = cmd.Flags().GetBool("json")
a.flagConfig, _ = cmd.Flags().GetString("config")
a.flagAPIKey, _ = cmd.Flags().GetString("api-key")
```

Unexported fields `flagJSON bool`, `flagConfig`, `flagAPIKey string` on App.

`version` command.

`cmd/fotoly/main.go` and `cmd/pixelfox/main.go` as above.

`Makefile`:

```
.PHONY: build test
VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)

build:
	mkdir -p bin
	go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT)" -o bin/fotoly ./cmd/fotoly
	go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT)" -o bin/pixelfox ./cmd/pixelfox

test:
	go test ./...
```

For unknown command: cobra returns error `unknown command "nope"`. Wrap in `Run`:

```go
func Run(a *App) int {
    cmd := a.Root()
    cmd.SetOut(a.Stdout)
    cmd.SetErr(a.Stderr)
    cmd.SilenceErrors = true
    if err := cmd.Execute(); err != nil {
        fmt.Fprintln(a.Stderr, err.Error())
        if isUsageErr(err) {
            return 2
        }
        return 1
    }
    return 0
}

func isUsageErr(err error) bool {
    if errors.Is(err, ErrUsage) {
        return true
    }
    msg := strings.ToLower(err.Error())
    return strings.Contains(msg, "unknown command") ||
        strings.Contains(msg, "unknown flag") ||
        strings.Contains(msg, "required flag") ||
        strings.Contains(msg, "accepts") ||
        strings.Contains(msg, "arg") && strings.Contains(msg, "received")
}
```

Keep `isUsageErr` in `root.go`. Tests cover unknown command → 2.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ui/ ./internal/cli/ -count=1 && go build -o /tmp/fotoly ./cmd/fotoly && go build -o /tmp/pixelfox ./cmd/pixelfox`

Expected: PASS, both binaries build. `/tmp/fotoly version` contains `fotoly`. `/tmp/pixelfox version` contains `pixelfox`.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/ internal/cli/ cmd/ Makefile go.mod go.sum
git commit -m "feat: add CLI skeleton with version and two brand binaries"
```

---

### Task 7: Setup command and huh prompter

**Files:**
- Create: `internal/cli/prompter.go`
- Create: `internal/cli/setup.go`
- Create: `internal/cli/setup_test.go`
- Modify: `internal/cli/root.go` (register `setup`)
- Modify: `internal/cli/app.go` (default Prompter)

**Interfaces:**
- Consumes: `config.Save`, `config.Reset`, `config.Load`, `config.FilePath`, `client.New`, `client.GetProfile`, `client.IsUnauthorized`, `Prompter`
- Produces: command `setup` with flags `--api-key` (already persistent) and `--reset`; aliases none

Huh form (`github.com/charmbracelet/huh`):

```go
func (HuhPrompter) PromptSetup(br brand.Brand, defaultURL string) (string, string, error) {
    key, base := "", defaultURL
    form := huh.NewForm(
        huh.NewGroup(
            huh.NewInput().Title("API key").EchoMode(huh.EchoModePassword).Value(&key),
            huh.NewInput().Title("Base URL").Value(&base),
        ),
    )
    if err := form.Run(); err != nil {
        return "", "", err
    }
    return strings.TrimSpace(key), strings.TrimSpace(base), nil
}
```

`ConfirmDelete` is a stub returning `false, fmt.Errorf("not implemented")` until Task 9 implements it with `huh.NewConfirm`.

- [ ] **Step 1: Write the failing tests**

Use httptest profile server + temp config dir.

```go
type stubPrompter struct {
    key, url string
    err      error
    confirm  bool
}

func (s stubPrompter) PromptSetup(brand.Brand, string) (string, string, error) {
    return s.key, s.url, s.err
}
func (s stubPrompter) ConfirmDelete(int) (bool, error) { return s.confirm, nil }
```

Tests:

1. `TestSetupNonTTYNoKeyExits1` — IsTTY false, no key, `setup` → Run code 1, stderr contains `No API key. Run 'fotoly setup'.` Wait: `setup` itself is how you set the key. Non-TTY `setup` without `--api-key` exits 1 with message `No API key. Run 'fotoly setup' --api-key …` **exact string:** `No API key. Use --api-key or run setup from a terminal.`

2. `TestSetupAPIKeyWritesConfig` — httptest profile 200 username `pixelpete` plan `free`; `SetArgs("setup", "--api-key", "pxl_test")`; after Execute, Load config from temp dir has that key; stdout contains `Logged in as pixelpete (free) on {srv.URL}`.

3. `TestSetupInvalidKey` — 401; command exits 1; config file not created (or key not saved).

4. `TestSetupReset` — write a config, `setup --reset`, file gone, exit 0.

5. `TestSetupWarnsNonPxlPrefix` — key `notaprefix`; stderr contains `pxl_`; still attempts profile (200) and saves.

6. `TestSetupTTYUsesPrompter` — IsTTY true, no --api-key, Prompter returns `pxl_from_ui` and custom URL; profile 200; config saved with that key.

If key does not start with `pxl_`, print to stderr: `Warning: API keys usually start with pxl_`

Success line exact: `Logged in as {username} ({plan}) on {base_url}`

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -count=1 -run TestSetup`

Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

Register `setup` on root.

Flow:

- `--reset`: `config.Reset(path)` where path is FlagConfig or FilePath(brand, userConfigDir). Print `Config removed.` to stdout. Return.

- Resolve key: persistent `--api-key` else env else (if TTY) Prompter else error `No API key. Use --api-key or run setup from a terminal.`

- Resolve base URL: prompter URL if prompted, else env `PREFIX_BASE_URL`, else existing file, else brand default. `--api-key` non-interactive does not change URL unless env/file set.

- `client.New(base, key, ua, a.HTTPClient).GetProfile`. On 401: `Invalid API key. Run 'fotoly setup'.`

- `config.Save(path, File{BaseURL: base, APIKey: key})`

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ ./internal/config/ ./internal/client/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/ go.mod go.sum
git commit -m "feat: add setup command that validates and stores the API key"
```

---

### Task 8: Auth gate and whoami

**Files:**
- Create: `internal/cli/whoami.go`
- Create: `internal/cli/whoami_test.go`
- Modify: `internal/cli/app.go` (helper `requireClient`)
- Modify: `internal/cli/root.go` (register `whoami`, alias `me`)

**Interfaces:**
- Consumes: `config.Load`, `client.GetProfile`, setup wizard from Task 7
- Produces: `whoami` command; `App.requireClient(cmd) (*client.Client, config.Values, error)`

`requireClient`:

1. Load config with Source from App flags/env/dir.
2. If APIKey empty and IsTTY: run the same setup flow as `setup` (prompter), reload. If still empty, error.
3. If APIKey empty and !IsTTY: error `No API key. Run 'fotoly setup'.` wrapped so Run returns 1 (not usage).
4. Return `client.New(vals.BaseURL, vals.APIKey, a.Brand.UserAgentString(a.Version), a.HTTPClient)`.

whoami human output (color headers optional), one field per line:

```
Username: pixelpete
Email: pete@example.com
Plan: premium
Images: 3
Storage: 100 B
```

`--json`: write the raw `UserAccount` as JSON to stdout (encode the struct). No progress on stdout.

On 401 after a saved key: print `Invalid API key. Run 'fotoly setup'.` exit 1.

- [ ] **Step 1: Write the failing tests**

1. `TestWhoamiNonTTYNoKey` — code 1, stderr `No API key. Run 'fotoly setup'.`
2. `TestWhoamiWithSavedKey` — save config, profile 200, stdout contains Username pixelpete.
3. `TestWhoamiJSON` — `--json`, stdout is JSON with `"username":"pixelpete"`.
4. `TestWhoamiTTYNoKeyRunsSetupThenProfile` — IsTTY true, empty config, Prompter returns valid key, profile 200, stdout has username, config written.
5. `TestWhoamiUnauthorized` — saved bad key, 401, stderr contains `Invalid API key`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -count=1 -run TestWhoami`

Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

Extract setup-from-values logic so `setup` command and `requireClient` share `a.runSetup(key, base string) error`.

Register `whoami` with `Aliases: []string{"me"}`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./... -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit -m "feat: add whoami and auto-setup when a command needs a key"
```

---

### Task 9: albums ls and images ls/get/delete

**Files:**
- Create: `internal/cli/albums.go`
- Create: `internal/cli/images.go`
- Create: `internal/cli/albums_test.go`
- Create: `internal/cli/images_test.go`
- Modify: `internal/cli/prompter.go` (`ConfirmDelete` via huh)
- Modify: `internal/cli/root.go`

**Interfaces:**
- Consumes: `requireClient`, `ListAlbums`, `ListImages`, `GetImage`, `DeleteImage`, `ResolveShareURL`, `Prompter.ConfirmDelete`
- Produces: `albums ls`; `images ls|get|delete`

`images ls` flags: `--limit` default 25; `--cursor`; `--album` int64; `--public` bool; `--private` bool; `--nsfw` bool; `--sfw` bool; `--tag` string.

If `--public` and `--private` both true → `fmt.Errorf("%w: --public and --private are mutually exclusive", ErrUsage)`. Same for `--nsfw`/`--sfw`.

Human `images ls` table columns: UUID (first 8 chars), file name, size, public, NSFW, share URL (resolved). If `has_more` and next_cursor set, stderr: `next cursor: {cursor}`.

`--json` prints the `ImageCollection` unchanged.

`images get <uuid>`: one uuid required. Human: uuid, view/share URL, original URL, nsfw, variants if present. `--json` = `ImageResource`.

`images delete <uuid...>`: at least one. If TTY and not `--yes`, `ConfirmDelete(n)`. If false, print `Aborted.` exit 0. If !TTY and not `--yes`, error `use --yes to delete` exit 1. Then delete each; per-id failures do not stop the rest; exit 1 if any failed.

`--json` delete: JSON array of objects `{image_uuid, status, message}` or `{image_uuid, error}`.

`albums ls`: table id, title, image count, public, share URL (resolved from `view_url`). `--json` = `AlbumCollection`.

Huh confirm:

```go
func (HuhPrompter) ConfirmDelete(n int) (bool, error) {
    ok := false
    title := fmt.Sprintf("Delete %d image(s)?", n)
    form := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title(title).Affirmative("Yes").Negative("No").Value(&ok)))
    if err := form.Run(); err != nil {
        return false, err
    }
    return ok, nil
}
```

- [ ] **Step 1: Write the failing tests**

httptest mux:

- GET `/api/v1/albums`
- GET `/api/v1/images`
- GET `/api/v1/images/{uuid}`
- DELETE `/api/v1/images/{uuid}` → 202

Tests:

1. `TestAlbumsLS` — stdout contains album title and id 42.
2. `TestImagesLSTruncatesUUID` — uuid `12345678-aaaa-bbbb-cccc-ddddeeeeffff` appears as `12345678` not full, unless `--json` which has full uuid.
3. `TestImagesLSPublicPrivateConflict` — args `images ls --public --private` → Run code 2.
4. `TestImagesGet` — prints resolved share URL.
5. `TestImagesDeleteRequiresYesNonTTY` — no `--yes`, IsTTY false, DELETE not called, exit 1.
6. `TestImagesDeleteYes` — `--yes`, DELETE called, stdout/stderr mentions accepted.
7. `TestImagesDeleteTTYDeclined` — Prompter confirm false, DELETE not called, `Aborted.`
8. `TestImagesLSJSONHasMoreCursorOnStderr` — has_more true, next_cursor `cur1`, `--json` stdout has next_cursor; without json, stderr has `next cursor: cur1`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -count=1 -run 'TestAlbums|TestImages'`

Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

Cobra structure:

```
imagesCmd := &cobra.Command{Use: "images", Short: "Manage images"}
imagesCmd.AddCommand(imagesLS, imagesGet, imagesDelete)
albumsCmd := &cobra.Command{Use: "albums", Short: "List albums"}
albumsCmd.AddCommand(albumsLS)
```

Size formatting: human bytes (`100 B`, `1.0 KB`, `1.5 MB`) — small helper in `ui.Bytes(n int64) string`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./... -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/ internal/ui/
git commit -m "feat: add albums ls and images ls/get/delete"
```

---

### Task 10: Upload command

**Files:**
- Create: `internal/cli/upload.go`
- Create: `internal/cli/upload_test.go`
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/app.go` (default Clipboard via `github.com/atotto/clipboard` `clipboard.WriteAll`; if it panics/fails, warn)

**Interfaces:**
- Consumes: `requireClient`, `CreateUploadSession`, `UploadFile`, `GetImageStatus`, `GetImage`, `ResolveShareURL`, `App.Sleep`, `App.Clipboard`, `App.IsTTY`
- Produces: `upload` (alias `up`)

Flags:

- `--album int64` default 0 (omit)
- `--nsfw` bool, only sent when true (`IsNSFW = &true`)
- `--processing string` default `default`; if not `default` or `original_only`, return `ErrUsage`
- `--no-wait` bool
- `--copy` bool
- `--no-copy` bool
- both copy flags → `ErrUsage`

Copy decision:

```
copyEnabled := a.IsTTY() && successCount == 1
if copyFlag { copyEnabled = true }
if noCopyFlag { copyEnabled = false }
```

Apply after the batch; if copyEnabled, clipboard the last successful URL. Clipboard error → stderr warning, command still 0 if uploads succeeded.

Per file:

1. Stat: missing/dir/size0 → that file fails, continue
2. Session POST
3. if max_bytes > 0 && size > max_bytes → fail file
4. if `--album` set && session.AlbumID == nil → stderr warning `album {id} was not bound; uploading without album`
5. UploadFile with progress to stderr only
6. unless no-wait: loop 45 times, Sleep(1s) between attempts (not before first). GetImageStatus. failed → file error. complete → break. after 45 → file error `Processing timed out for {uuid}; check later with images get`
7. GetImage, ResolveShareURL(base, view from image, view from upload, url from image)
8. duplicate from upload response

Zero files: `ErrUsage` `upload requires at least one file`.

JSON stdout: array of

```json
{"file":"cat.jpg","ok":true,"image_uuid":"...","url":"https://...","duplicate":false}
```

Human: print each URL; end with `uploaded N, failed M` on stderr if more than one file or any failed.

Exit 1 if any failed.

Progress: if stderr TTY, write `\r{name} {percent:.1f}%` then newline at 100. If not TTY, print a line every 25% or at 100. Do not write progress to stdout.

- [ ] **Step 1: Write the failing tests**

Mux:

- POST `/api/v1/upload/sessions` → `{upload_url: srv.URL+"/api/v1/upload", token:"tok", pool_id:1, expires_at: 9999999999, max_bytes: 10_000_000}`
- When album requested with id 42, include `"album_id":42`. When requested id 99, omit album_id.
- POST `/api/v1/upload` → `{image_uuid:"img-1", view_url:"/i/share1", duplicate:false}`
- GET `/api/v1/images/img-1/status` → first call complete true
- GET `/api/v1/images/img-1` → `{image_uuid:"img-1", view_url:"/i/share1"}`

Create a tiny temp file with a few bytes.

Tests:

1. `TestUploadHappyPath` — stdout contains `http://127.0.0.1` and `/i/share1`. Session received `file_size`.
2. `TestUploadJSON` — `--json` stdout is JSON array ok true.
3. `TestUploadNoWaitSkipsStatus` — `--no-wait`; status path not hit; stdout/json has uuid.
4. `TestUploadMissingFileContinues` — args `missing.jpg` and good file; exit 1; good file still uploaded (assert upload POST count == 1).
5. `TestUploadProcessingInvalid` — `--processing custom` exit 2.
6. `TestUploadCopyAndNoCopyConflict` — exit 2.
7. `TestUploadClipboardSingleTTY` — IsTTY true, one file, Clipboard func records the URL.
8. `TestUploadAlbumNotBoundWarns` — `--album 99`, stderr contains `not bound`.
9. `TestUploadZeroFiles` — exit 2.
10. `TestUploadProcessingTimeout` — status always `{complete:false,failed:false}`; Sleep is no-op; error contains `timed out`; GetImageStatus called 45 times.

For timeout test, do not Sleep really; `a.Sleep = func(time.Duration) {}`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -count=1 -run TestUpload`

Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

`upload.go` as specified. Alias `up`.

Default processing body always sent as `{"profile":"default"}` or `original_only` so the CMS snapshot is explicit.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./... -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/ go.mod go.sum
git commit -m "feat: add upload with session, progress, and processing poll"
```

---

### Task 11: README and make smoke

**Files:**
- Modify: `README.md`
- Modify: `Makefile` if needed

**Interfaces:**
- Consumes: all commands from previous tasks
- Produces: user-facing README matching actual flags

- [ ] **Step 1: Write README**

Replace the one-liner. English. Include:

- What it is (official CLI for fotoly.eu and pixelfox.cc)
- Install: `go install github.com/ManuelReschke/fotoly-cli/cmd/fotoly@latest` and the pixelfox equivalent
- First run: `fotoly whoami` launches setup; or `fotoly setup --api-key pxl_…`
- Config paths (`~/.config/fotoly/config.toml` / pixelfox)
- Env vars
- Command examples: upload, images ls, images delete --yes, albums ls, `--json`
- Two binaries, separate configs
- Link to API settings on the website (`/user/settings`)

Do not claim features that are non-goals (download, album create, TUI).

- [ ] **Step 2: Verify binaries still build and tests pass**

Run: `go test ./... -count=1 && make build && ./bin/fotoly version && ./bin/pixelfox version && ./bin/fotoly --help && ./bin/pixelfox --help`

Expected: tests PASS; version lines include each binary name; help lists `setup`, `whoami`, `upload`, `images`, `albums`.

- [ ] **Step 3: Commit**

```bash
git add README.md Makefile
git commit -m "docs: add CLI usage README for fotoly and pixelfox"
```

---

## Self-review (plan vs spec)

| Spec item | Task |
|---|---|
| Two binaries, one module | 1, 6 |
| Brand hosts, config dirs, env prefixes, accents, UA | 1 |
| TOML 0600, flag > env > file, reset | 2, 7 |
| X-API-Key on API, Bearer on upload | 3, 5 |
| Profile / whoami / JSON passthrough | 3, 8 |
| Share URL resolve | 3, 9, 10 |
| Albums ls, images ls/get/delete | 4, 9 |
| Upload session, multipart, poll 1s×45, duplicate, album warn | 5, 10 |
| Setup wizard, TTY auto-setup, non-TTY message, pxl_ warning | 7, 8 |
| `--json` stdout vs stderr progress | 8–10 |
| `--copy` / `--no-copy`, clipboard best-effort | 10 |
| `--public`/`--private` usage error | 9 |
| `--processing` default\|original_only | 10 |
| NO_COLOR / TTY | 6 |
| Exit 0/1/2 | 6–10 |
| README `go install` only | 11 |
| No live network tests | 2–10 |
| Non-goals excluded | — not scheduled |
