# Upload Processing-Wait Visualization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show a stderr processing indicator (TTY spinner + elapsed seconds, or one non-TTY line) while `upload` polls image status after the file POST.

**Architecture:** Keep the existing 45×1s `waitForImage` poll. After the first pending status, start a `processingWait` helper that writes stderr. TTY redraws `\rprocessing {name} {spinner} {Ns}` every 100ms (the 1s gap is ten `App.Sleep(100ms)` slices). Success overwrites the line with `done` and a newline; failure/timeout prints a newline first so the error is not eaten. No processing output when the first poll is already complete or when `--no-wait` is set. No client or API changes.

**Tech Stack:** Go 1.26, existing Cobra CLI, `fmt`/`time` only (no Charm spinner). Spec: `docs/superpowers/specs/2026-09-13-upload-processing-wait-design.md`.

## Global Constraints

- English UI only (`processing`, `done`, existing failure/timeout copy unchanged)
- Status poll: 1s interval, 45 attempts
- Spinner frames: `|/-\`, one frame every 100ms
- Elapsed: wall-clock integer seconds since wait started
- Progress and processing lines: stderr only; `--json` stdout stays a JSON array
- `--no-wait`: no status poll, no processing line
- No new packages or CLI flags
- Code stays in `internal/cli/upload.go`
- `App.Sleep` is the test seam (existing tests use a no-op)
- No live HTTP to fotoly.eu / pixelfox.cc in tests
- Exit codes: 0 success, 1 runtime/API/validation, 2 usage

---

## File map

Modify:

- `internal/cli/upload.go` — pass basename into `waitForImage`; add `processingWait`; split the 1s sleep into 100ms ticks
- `internal/cli/upload_test.go` — harness fields for delayed complete / failed status; new tests listed below

Do not modify `internal/client`, flags, or README (global `--json` line already says progress is on stderr).

---

### Task 1: Processing-wait indicator on upload status poll

**Files:**
- Modify: `internal/cli/upload.go`
- Test: `internal/cli/upload_test.go`

**Interfaces:**
- Consumes: `App.Sleep`, `App.isTTY`, `App.Stderr`, `client.Client.GetImageStatus`, `statusPollAttempts` (45)
- Produces:
  - `func (a *App) waitForImage(cmd *cobra.Command, c *client.Client, uuid, name string) error`
  - `processingWait` with `begin()`, `tick()`, `succeed()`, `fail()`
  - constants `processingSpinner = "|/-\\"`, `processingTick = 100 * time.Millisecond`

- [ ] **Step 1: Extend the upload harness and write failing tests**

In `internal/cli/upload_test.go`, add fields to `uploadHarness`:

```go
type uploadHarness struct {
	srv                   *httptest.Server
	mu                    sync.Mutex
	sessionBody           client.UploadSessionRequest
	uploadCount           int
	statusCount           int
	statusPending         bool
	statusFailed          bool
	pendingBeforeComplete int
}
```

Change the status handler so the first `pendingBeforeComplete` calls return pending, `statusFailed` returns failed, and `statusPending` still means forever-pending:

```go
mux.HandleFunc("GET /api/v1/images/{uuid}/status", func(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	h.statusCount++
	count := h.statusCount
	pendingForever := h.statusPending
	failed := h.statusFailed
	before := h.pendingBeforeComplete
	h.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if failed {
		_, _ = io.WriteString(w, `{"complete":false,"failed":true}`)
		return
	}
	if pendingForever || count <= before {
		_, _ = io.WriteString(w, `{"complete":false,"failed":false}`)
		return
	}
	_, _ = io.WriteString(w, `{"complete":true,"failed":false}`)
})
```

Add these tests (keep `TestUploadProcessingTimeout` as-is except it must still see 45 status calls; its stderr may now include a non-TTY `processing` line):

```go
func TestUploadProcessingTTYPendingThenDone(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	h := newUploadHarness(t)
	h.pendingBeforeComplete = 1
	attachServer(a, h.srv)
	saveTestAPIKey(t, a, h.srv.URL)
	path := writeTempUpload(t, "cat.jpg", "hello")
	a.IsTTY = func() bool { return true }

	a.Root().SetArgs([]string{"upload", path})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "processing cat.jpg") {
		t.Fatalf("stderr=%q", errOut)
	}
	if !strings.Contains(errOut, "done") {
		t.Fatalf("missing done: %q", errOut)
	}
	if !strings.Contains(stdout.String(), "/i/share1") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestUploadProcessingNonTTYOneLine(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	h := newUploadHarness(t)
	h.pendingBeforeComplete = 1
	attachServer(a, h.srv)
	saveTestAPIKey(t, a, h.srv.URL)
	path := writeTempUpload(t, "cat.jpg", "hello")

	a.Root().SetArgs([]string{"upload", path})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	errOut := stderr.String()
	if strings.Count(errOut, "processing cat.jpg …") != 1 {
		t.Fatalf("want one start line, stderr=%q", errOut)
	}
	if strings.Contains(errOut, "done") {
		t.Fatalf("non-TTY must not print done: %q", errOut)
	}
}

func TestUploadProcessingSkippedWhenAlreadyComplete(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	h := newUploadHarness(t)
	attachServer(a, h.srv)
	saveTestAPIKey(t, a, h.srv.URL)
	path := writeTempUpload(t, "cat.jpg", "hello")

	a.Root().SetArgs([]string{"upload", path})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "processing") {
		t.Fatalf("no wait: stderr=%q", stderr.String())
	}
}

func TestUploadNoWaitHasNoProcessingLine(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	h := newUploadHarness(t)
	attachServer(a, h.srv)
	saveTestAPIKey(t, a, h.srv.URL)
	path := writeTempUpload(t, "cat.jpg", "hello")

	a.Root().SetArgs([]string{"upload", "--no-wait", path})
	code := Run(a)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if h.statuses() != 0 {
		t.Fatalf("status calls=%d", h.statuses())
	}
	if strings.Contains(stderr.String(), "processing") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestUploadProcessingFailedNoSpinner(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	h := newUploadHarness(t)
	h.statusFailed = true
	attachServer(a, h.srv)
	saveTestAPIKey(t, a, h.srv.URL)
	path := writeTempUpload(t, "cat.jpg", "hello")
	a.IsTTY = func() bool { return true }

	a.Root().SetArgs([]string{"upload", path})
	code := Run(a)
	if code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "failed") {
		t.Fatalf("stderr=%q", errOut)
	}
	if strings.Contains(errOut, "processing") {
		t.Fatalf("first-poll fail must not spin: %q", errOut)
	}
}

func TestUploadProcessingTimeoutTTYDoesNotEatError(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	h := newUploadHarness(t)
	h.statusPending = true
	attachServer(a, h.srv)
	saveTestAPIKey(t, a, h.srv.URL)
	path := writeTempUpload(t, "cat.jpg", "hello")
	a.IsTTY = func() bool { return true }
	a.Sleep = func(time.Duration) {}

	a.Root().SetArgs([]string{"upload", path})
	code := Run(a)
	if code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if h.statuses() != 45 {
		t.Fatalf("GetImageStatus calls=%d", h.statuses())
	}
	for _, line := range strings.Split(stderr.String(), "\n") {
		if strings.Contains(line, "timed out") && strings.Contains(line, "processing") {
			t.Fatalf("timeout eaten by spinner: %q", line)
		}
	}
	if !strings.Contains(stderr.String(), "timed out") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestUploadProcessingJSONStaysOnStderr(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	h := newUploadHarness(t)
	h.pendingBeforeComplete = 1
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
		t.Fatalf("stdout not JSON: %v %q", err, stdout.String())
	}
	if strings.Contains(stdout.String(), "processing") {
		t.Fatalf("processing leaked to stdout: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "processing cat.jpg …") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}
```

Also assert in `TestUploadProcessingTimeout` (non-TTY, already present) that status is still called 45 times. No change required if the loop count is unchanged.

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/cli/ -count=1 -run 'TestUploadProcessing|TestUploadNoWaitHasNoProcessingLine|TestUploadProcessingSkippedWhenAlreadyComplete'
```

Expected: FAIL because stderr has no `processing` / `done` lines yet (`waitForImage` is silent). `TestUploadProcessingFailedNoSpinner` may already pass (no processing line exists); that is fine. The TTY pending-then-done, non-TTY, JSON, and no-wait processing-line tests must fail.

- [ ] **Step 3: Implement `processingWait` and wire it into `waitForImage`**

In `internal/cli/upload.go`, add `"strings"` to imports and replace the poll constants / `waitForImage` as follows.

```go
const (
	statusPollAttempts = 45
	processingSpinner  = `|/-\`
	processingTick     = 100 * time.Millisecond
)

func (a *App) waitForImage(cmd *cobra.Command, c *client.Client, uuid, name string) error {
	wait := &processingWait{w: a.Stderr, tty: a.isTTY(), name: name}
	ticks := int(time.Second / processingTick)
	for i := 0; i < statusPollAttempts; i++ {
		if i > 0 {
			for t := 0; t < ticks; t++ {
				if a.Sleep != nil {
					a.Sleep(processingTick)
				}
				wait.tick()
			}
		}
		st, err := c.GetImageStatus(cmd.Context(), uuid)
		if err != nil {
			wait.fail()
			return err
		}
		if st.Failed {
			wait.fail()
			return fmt.Errorf("Processing failed for %s", uuid)
		}
		if st.Complete {
			wait.succeed()
			return nil
		}
		if !wait.started {
			wait.begin()
		}
	}
	wait.fail()
	return fmt.Errorf("Processing timed out for %s; check later with images get", uuid)
}

type processingWait struct {
	w       io.Writer
	tty     bool
	name    string
	start   time.Time
	frame   int
	width   int
	started bool
}

func (p *processingWait) begin() {
	p.started = true
	p.start = time.Now()
	if p.tty {
		p.draw(false)
		return
	}
	fmt.Fprintf(p.w, "processing %s …\n", p.name)
}

func (p *processingWait) tick() {
	if p.started && p.tty {
		p.draw(false)
	}
}

func (p *processingWait) succeed() {
	if !p.started {
		return
	}
	if p.tty {
		p.draw(true)
		fmt.Fprintln(p.w)
	}
}

func (p *processingWait) fail() {
	if p.started && p.tty {
		fmt.Fprintln(p.w)
	}
}

func (p *processingWait) draw(done bool) {
	var s string
	if done {
		s = fmt.Sprintf("processing %s done", p.name)
	} else {
		s = fmt.Sprintf("processing %s %c %ds", p.name, processingSpinner[p.frame%len(processingSpinner)], int(time.Since(p.start).Seconds()))
		p.frame++
	}
	if len(s) < p.width {
		s += strings.Repeat(" ", p.width-len(s))
	} else {
		p.width = len(s)
	}
	fmt.Fprintf(p.w, "\r%s", s)
}
```

Add `"io"` to the import list (needed by `processingWait.w`).

Change the call site in `uploadOne`:

```go
if err := a.waitForImage(cmd, c, res.ImageUUID, filepath.Base(path)); err != nil {
```

Do not change poll count, error strings, `--no-wait` short-circuit, or `UploadFile` progress.

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/cli/ -count=1
go test ./... -count=1
```

Expected: PASS. `TestUploadProcessingTimeout` still reports 45 status calls.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/upload.go internal/cli/upload_test.go
git commit -m "feat: show processing spinner while waiting after upload"
```
