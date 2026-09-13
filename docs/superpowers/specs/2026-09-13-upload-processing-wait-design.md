# Design: Upload processing-wait visualization

**Date:** 2026-09-13  
**Status:** Draft (awaiting user review)  
**Repo:** `github.com/ManuelReschke/fotoly-cli`  
**Parent:** [2026-09-08-fotoly-cli-design.md](2026-09-08-fotoly-cli-design.md)

## Problem

`fotoly upload` already prints byte-transfer percent on stderr. After the POST, `waitForImage` polls `GET /images/{uuid}/status` every 1s (max 45 attempts) with no output. On slow processing the CLI looks hung. The status payload is only `{complete, failed, view_url}` — there is no processing percent to show.

## Goals

- Show that processing is in progress while the existing poll loop runs.
- Match the current upload-percent style: stderr only, `\r` on TTY, quiet when not a TTY.
- Keep poll semantics, `--no-wait`, `--json` stdout, and English copy unchanged.

## Non-goals

- Fake percent from poll index (`3/45`).
- Charm / bubbles spinner or a progress bar.
- Changing the 1s / 45-attempt budget.
- Visualizing wait when `--no-wait` is set.
- New packages or new CLI flags.

## Decisions

| Topic | Choice |
|---|---|
| TTY line | `processing {basename} {spinner} {Ns}` overwritten with `\r` |
| Spinner | `\|/-\\`, one frame every 100ms |
| Elapsed | Wall-clock seconds since wait started, integer (`4s`) |
| Success | Same line becomes `processing {basename} done`, then newline. Pad or erase to end of line so a longer `Ns` frame cannot leave leftover characters. |
| Failure / timeout | Newline first, then the existing error (so `\r` cannot eat it) |
| Non-TTY | One start line `processing {basename} …`, then silent |
| First poll complete | No processing line (nothing to wait for) |
| `--json` | These lines stay on stderr; stdout stays the JSON array |
| `--no-wait` | No status poll, no processing line |
| Code | Stay in `internal/cli/upload.go`; pass basename into `waitForImage` |
| Sleep | Split the 1s poll gap into 100ms `App.Sleep` slices so tests with a no-op `Sleep` stay instant |

## Behavior

Per file, after `UploadFile` returns an `image_uuid`, unless `--no-wait`:

1. Call `GetImageStatus` immediately (attempt 1 of 45).
2. If `complete` on that first call, return with no processing output.
3. If `failed`, return the existing `Processing failed for {uuid}` error (no spinner).
4. Otherwise print the processing indicator and continue the existing loop: sleep 1s, poll, up to 45 attempts total.
5. On later `complete`: finish the TTY line as `done`.
6. On later `failed` or timeout: newline (TTY), then the existing error (`Processing failed for …` / `Processing timed out for {uuid}; check later with images get`).

Filename is `filepath.Base` of the local path, same as upload percent.

Example TTY sequence:

```
cat.jpg 100.0%
processing cat.jpg | 4s
```

then the line becomes:

```
processing cat.jpg done
https://fotoly.eu/i/…
```

Non-TTY:

```
cat.jpg 100.0%
processing cat.jpg …
https://fotoly.eu/i/…
```

`--json` still encodes the result array on stdout. Processing lines never go to stdout.

## Implementation

Extend `waitForImage` in `internal/cli/upload.go`. Do not change `internal/client` or the status JSON type.

While waiting after the first pending poll, the 1s interval is ten `App.Sleep(100 * time.Millisecond)` calls, redrawing the TTY spinner each slice. Elapsed seconds use wall clock (`time.Since`), not poll count, so a slow HTTP round-trip still counts.

`App.Sleep` remains the test seam. Existing timeout tests that set `Sleep` to a no-op stay fast; they may write extra spinner frames to stderr, which is fine if assertions stay substring-based.

## Tests

Keep existing upload tests passing. Add coverage in `internal/cli/upload_test.go`:

| Case | Expect |
|---|---|
| TTY, first status pending then complete | stderr contains `processing` and the basename, and ends the wait with `done` |
| Non-TTY, pending then complete | one `processing cat.jpg …` line; no spinner-frame spam |
| First poll already complete | no `processing` line |
| `--no-wait` | no status poll, no `processing` line |
| Timeout (status always pending) | TTY newline before the existing `timed out` message; still 45 `GetImageStatus` calls |

The current timeout test uses a no-op `Sleep` and must keep calling status 45 times.

## Out of scope later

If the API later adds a processing percent, replace the spinner with a real percent. Until then do not invent one.
