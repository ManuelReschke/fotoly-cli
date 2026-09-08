# Design: fotoly-cli (Fotoly / PixelFox CLI)

**Date:** 2026-09-08  
**Status:** Draft (awaiting user review)  
**Repo:** `github.com/ManuelReschke/fotoly-cli`  
**CMS API:** PixelFox / Fotoly public API v1 (`/api/v1`)

## Problem

fotoly.eu and pixelfox.cc share one CMS. There is a public API (API key, upload sessions, images, albums, profile) and a browser extension that uses it, but no official command-line tool. Users cannot script uploads, list their library, or manage keys from a terminal.

## Goals

- Ship one Go codebase as two binaries: `fotoly` and `pixelfox`.
- First-run (or missing key) walks through an interactive setup that stores the API key.
- v1 covers setup, account, upload, image list/get/delete, and album list (as upload target).
- Hybrid UX: Cobra commands for scripts, Charm (huh / lipgloss / progress) for setup and human output.
- English UI. `--json` for automation.
- No live calls to production in tests.

## Non-goals (v1)

- Full-screen Bubble Tea app
- Image metadata edit (PATCH title/tags/public/NSFW)
- Download of originals or variants
- Album create/update/delete
- Recursive directory upload, stdin (`-`) upload, custom processing derivatives
- Email/password login or app-session tokens (`pxls_`)
- OS keyring
- Goreleaser / package managers (document `go install` only)
- Generated OpenAPI client

## Decisions

| Topic | Choice |
|---|---|
| UX | Hybrid: Cobra + Charm wizard/tables/progress |
| v1 scope | Upload-first: setup, whoami, upload, images ls/get/delete, albums ls |
| Binaries | Two commands, one module: `cmd/fotoly`, `cmd/pixelfox` |
| Config | TOML under `os.UserConfigDir()/<app>/config.toml`, mode `0600` |
| Config vs env vs flags | Flag > env > file |
| Language | English |
| Auth | User API key only (`X-API-Key`, keys typically `pxl_…`) |
| HTTP | `net/http`, hand-written client matching OpenAPI v1 |
| Processing default | `default` (CMS user settings). Flag: `default` or `original_only` |
| Go version | 1.26 (same as PixelFox CMS) |

## Brand

`internal/brand` holds a value object. Each `main` injects one:

| Field | `fotoly` | `pixelfox` |
|---|---|---|
| Display name | Fotoly | PixelFox |
| Binary | `fotoly` | `pixelfox` |
| Default base URL | `https://fotoly.eu` | `https://pixelfox.cc` |
| API root | `{base}/api/v1` | `{base}/api/v1` |
| Config app dir | `fotoly` | `pixelfox` |
| Env prefix | `FOTOLY` | `PIXELFOX` |
| User-Agent | `fotoly-cli/<version>` | `pixelfox-cli/<version>` |

`base_url` may be overridden to `http://localhost:8080` or `http://127.0.0.1:8080` for local CMS. Other hosts are allowed if the user sets them explicitly (self-hosted / staging); the CLI does not pin an allowlist.

`cmd/fotoly/main.go` and `cmd/pixelfox/main.go` only call `cli.Execute(brand)`. All commands live in `internal/cli`.

## Layout

```
fotoly-cli/
  cmd/fotoly/main.go
  cmd/pixelfox/main.go
  internal/
    brand/      Brand value + the two presets
    config/     load/save TOML, permissions, env overlay
    client/     API v1 HTTP client
    cli/        Cobra root and subcommands
    ui/         huh wizard, lipgloss tables, progress, color/NO_COLOR
  docs/superpowers/specs/2026-09-08-fotoly-cli-design.md
```

Go module path: `github.com/ManuelReschke/fotoly-cli`.

## Config

Path: `filepath.Join(os.UserConfigDir(), brand.ConfigApp, "config.toml")`.

On Linux that is typically `~/.config/fotoly/config.toml` and `~/.config/pixelfox/config.toml`. The two brands never share a file.

```toml
base_url = "https://fotoly.eu"
api_key  = "pxl_…"
```

- Create the directory with `0700`, the file with `0600`.
- `setup --reset` deletes that brand's config file (not the other brand). Missing file is success.
- Env (prefix per brand): `{PREFIX}_API_KEY`, `{PREFIX}_BASE_URL`, `{PREFIX}_CONFIG` (explicit file path).
- Persistent flag `--config` overrides the path.
- Persistent flag `--api-key` overrides the key for one invocation and is not written unless used with `setup`.
- Missing file is not an error until a command needs a key.

## Commands

Same tree for both binaries. Examples use `fotoly`.

| Command | Auth | Behavior |
|---|---|---|
| `fotoly` (no args) | no | Cobra help. Does not start setup. |
| `fotoly setup` | no | Interactive wizard (TTY) or non-interactive flags. |
| `fotoly setup --api-key pxl_…` | no | Validate, write config, print username. |
| `fotoly setup --reset` | no | Delete config file. |
| `fotoly whoami` (`me`) | yes | `GET /user/profile`. Human: username, email, plan, image count, storage used/limit. |
| `fotoly upload <files…>` (`up`) | yes | Session → POST file → poll → print share URL. Zero files is usage error (exit 2). |
| `fotoly images ls` | yes | `GET /images` table. |
| `fotoly images get <uuid>` | yes | `GET /images/{uuid}` details + variant URLs. |
| `fotoly images delete <uuid…>` | yes | `DELETE /images/{uuid}` per id. TTY confirm unless `--yes`. |
| `fotoly albums ls` | yes | `GET /albums` table (id, title, image count, public, share URL). |
| `fotoly version` | no | Binary name, version, commit (ldflags). |

Global persistent flags: `--json`, `--config`, `--api-key`. `--json` sends machine JSON to stdout and progress/prompts to stderr.

Auth-required commands with no key:

- TTY: run the setup wizard, then continue the original command if setup succeeds.
- Non-TTY: exit 1, message `run 'fotoly setup'`.

### Setup wizard (TTY)

Charm `huh`:

1. Brand banner (name + default host, accent color).
2. Password-style API key input. Trim whitespace. Empty is invalid.
3. Optional base URL; default is the brand host. Shown as one field, prefilled.
4. `GET /user/profile` with the candidate key.
5. On success: write config, print `Logged in as {username} ({plan}) on {base_url}`.
6. On 401: stay in the wizard with `Invalid API key`. Other errors abort with the API message.

Non-interactive setup requires `--api-key` (or `{PREFIX}_API_KEY`). No TTY + no key → exit 1.

Do not reject keys that lack the `pxl_` prefix; the server is the source of truth. If the key does not start with `pxl_`, print a warning to stderr and still try.

### Upload flags

| Flag | Default | Notes |
|---|---|---|
| `--album <id>` | none | Existing album owned by the user. |
| `--nsfw` | omit (CMS default) | Sends `"is_nsfw": true`. There is no `--no-nsfw`; omit the flag to use CMS preference. |
| `--processing` | `default` | `default` or `original_only`. Other values are a usage error. |
| `--no-wait` | false | Skip status poll; print UUID from the upload response. |
| `--copy` | off unless TTY and exactly one file succeeds | Force clipboard even for a multi-file batch (copies the last successful URL). |
| `--no-copy` | — | Never copy. If both `--copy` and `--no-copy` are set, usage error (exit 2). |

Globs are expanded by the shell. A path that is a directory or missing is a per-file error. v1 does not walk directories.

### `images ls` flags

`--limit` (default 25, max 100), `--cursor`, `--album`, `--public` / `--private` (maps to `is_public`), `--nsfw` / `--sfw` (`is_nsfw`), `--tag`. `--public` with `--private`, or `--nsfw` with `--sfw`, is a usage error (exit 2). Human table columns: UUID, file name, size, public, NSFW, share URL. If `has_more`, print `next cursor: …` on stderr (JSON keeps `next_cursor` from the API).

## Client

Package `internal/client`. One `Client` constructed with `baseURL`, `apiKey`, `userAgent`, `http.Client`.

Timeout: 30s per request except the file upload, which uses a longer timeout (at least 10 minutes) or no extra timeout beyond context cancel.

Headers on API calls:

- `Accept: application/json`
- `X-API-Key: {key}`
- `User-Agent: {brand user agent}`

Upload POST to `upload_url` uses `Authorization: Bearer {session token}` and must **not** send the user API key. Multipart field name: `file`.

Endpoints used in v1:

| Method | Path | Used by |
|---|---|---|
| GET | `/user/profile` | setup, whoami |
| POST | `/upload/sessions` | upload |
| POST | `{upload_url}` | upload (often `/api/v1/upload`) |
| GET | `/images/{uuid}/status` | upload wait |
| GET | `/images/{uuid}` | upload finish, `images get` |
| GET | `/images` | `images ls` |
| DELETE | `/images/{uuid}` | `images delete` |
| GET | `/albums` | `albums ls` |

JSON types follow `public/docs/v1/openapi.yml` in the PixelFox repo. Do not invent field names.

Share URL resolution (same as the browser extension): take `image.view_url` or upload `view_url` or `image.url`, then `url.Parse` relative to `base_url` so a path `/i/{share}` becomes `https://fotoly.eu/i/{share}`.

## Upload pipeline

Per file, sequential (not parallel):

1. `os.Stat`. Reject missing paths, directories, and size `0`.
2. `POST /upload/sessions` with `{"file_size": N}` plus optional `album_id`, `is_nsfw`, `processing: {profile}`.
3. If session `max_bytes` is set and file size exceeds it, fail that file with a quota/limit message without uploading. If `--album` was set and the session response omits `album_id`, warn on stderr that the album was not bound (CMS ignores unknown/foreign IDs) and continue the upload.
4. Multipart POST to `upload_url` with progress on stderr (bytes, percent). Human mode uses a Charm progress bar when stderr is a TTY; otherwise print periodic percent lines.
5. Unless `--no-wait`: poll `GET /images/{uuid}/status` every **1s**, max **45** attempts (same as the extension). `failed=true` → per-file error. Timeout → per-file error (`processing timed out; check later with images get`).
6. `GET /images/{uuid}` and print the resolved share URL. If the upload response has `duplicate: true`, mark it in the output (`duplicate` column / JSON field).
7. Single successful file + TTY + copy enabled → write share URL to clipboard.

Batch: one file failure does not stop the rest. Exit 1 if any file failed. Human summary: `uploaded N, failed M`. JSON stdout is an array of per-file objects:

```json
[
  {"file": "cat.jpg", "ok": true, "image_uuid": "…", "url": "https://fotoly.eu/i/…", "duplicate": false},
  {"file": "notes.txt", "ok": false, "error": "unsupported media type"}
]
```

## Errors

Map HTTP status to a short English line; always include the API `message` when present.

| Status / case | User-facing meaning |
|---|---|
| no key, non-TTY | `No API key. Run 'fotoly setup'.` |
| 401 | `Invalid API key. Run 'fotoly setup'.` |
| 403 | account not allowed / inactive (API message) |
| 413 | file exceeds plan or quota (API message) |
| 415 | not an image the CMS accepts |
| 429 | retry once after `Retry-After` or 2s, then fail |
| 5xx / network | retry once, then fail with host + error |
| processing failed | `Processing failed for {uuid}` |
| processing timeout | `Processing timed out for {uuid}; check later with images get` (object may already exist on the server) |

`--json` on a command-level failure (not a mixed upload batch): JSON error object on stderr, empty or omitted stdout, exit ≠ 0.

`--json` on read commands is the API response body unchanged: `whoami` → `UserAccount`, `images ls` → `ImageCollection`, `images get` → `ImageResource`, `albums ls` → `AlbumCollection`, `images delete` → per-id array of `ImageDeletionAccepted` or `{uuid, error}`.

Color: lipgloss when stderr/stdout is a TTY and `NO_COLOR` is unset. Otherwise plain text.

Exit codes: `0` success, `1` runtime/API/validation, `2` usage (Cobra).

## UI

- Accent color per brand for the banner and table headers only: Fotoly `#E85D04`, PixelFox `#F97316`.
- Tables via lipgloss. Truncate UUID in tables to 8 chars unless `--json`.
- `huh` only for setup and delete confirmation. No TUI navigator.
- Delete confirm: `Delete {n} image(s)?` default no.

## Testing

All tests use `httptest` or temp dirs. No network to fotoly.eu / pixelfox.cc.

- `config`: round-trip TOML; file mode `0600`; env overrides file; `FOTOLY_*` does not leak into a PixelFox brand client.
- `client`: fake server for profile, session, upload, status (complete / failed / slow), list, delete; assert `X-API-Key` on API calls and `Authorization: Bearer` on upload; 401/413/415 bodies.
- `cli`: non-TTY setup without key exits 1; `setup --api-key` writes config; upload happy path prints share URL; batch with one bad path still uploads the good file and exits 1.
- `brand`: each binary's default host and config dir differ.

## Implementation notes

- Version via ldflags: `main.version`, `main.commit`. Default `dev`.
- Clipboard: optional dependency; if the OS has no clipboard, `--copy` warns and continues.
- Do not log or print the full API key. Wizard input is masked. `whoami` does not echo the key; at most the stored prefix is unnecessary — omit it.
- Keep `internal/client` free of Cobra and Charm so tests construct it directly.

## Success

v1 is done when:

1. `go build ./cmd/fotoly` and `go build ./cmd/pixelfox` produce two binaries with different defaults.
2. First `fotoly whoami` on a clean machine opens setup, stores the key, then shows the profile.
3. `fotoly upload image.jpg` prints a share URL after processing.
4. `pixelfox` uses its own config file and host.
5. `go test ./...` passes offline.
