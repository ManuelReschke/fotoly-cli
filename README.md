# fotoly-cli

Official command-line tools for [fotoly.eu](https://fotoly.eu) and [pixelfox.cc](https://pixelfox.cc). One Go module ships two binaries — `fotoly` and `pixelfox` — with separate configs, hosts, and environment prefixes.

Use them to set up an API key, check your account, upload images, list, inspect, edit, or delete images, and list albums.

## Install

On macOS or Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/ManuelReschke/fotoly-cli/main/install.sh | sh
```

On Windows:

```powershell
powershell -ExecutionPolicy ByPass -c "irm https://raw.githubusercontent.com/ManuelReschke/fotoly-cli/main/install.ps1 | iex"
```

The installer downloads the latest release for your machine, checks it against `SHA256SUMS`, and installs both `fotoly` and `pixelfox`. The default directory is `~/.local/bin` (`%USERPROFILE%\.local\bin` on Windows). A root install on macOS or Linux uses `/usr/local/bin`.

Pin a release or choose the directory:

```bash
curl -fsSL https://raw.githubusercontent.com/ManuelReschke/fotoly-cli/main/install.sh | sh -s -- --version 1.0.0
FOTOLY_INSTALL_DIR=$HOME/bin sh install.sh
```

On Windows, set `$env:FOTOLY_VERSION` or `$env:FOTOLY_INSTALL_DIR` before the installer command.

With Go 1.26+ you can still install from source:

```bash
go install github.com/ManuelReschke/fotoly-cli/cmd/fotoly@latest
go install github.com/ManuelReschke/fotoly-cli/cmd/pixelfox@latest
```

Or build from a clone:

```bash
make build   # produces ./bin/fotoly and ./bin/pixelfox
```

## API key

Create a user API key in the website settings:

- Fotoly: https://fotoly.eu/user/settings
- PixelFox: https://pixelfox.cc/user/settings

Keys usually start with `pxl_`.

## First run

On a TTY, a missing key starts the setup wizard (for example when you run `fotoly whoami`). You can also configure non-interactively:

```bash
fotoly setup --api-key pxl_…
# or
pixelfox setup --api-key pxl_…
```

Reset stored config for that brand only:

```bash
fotoly setup --reset
```

## Config

Each binary uses its own TOML file (mode `0600`):

| Binary     | Typical path (Linux)              |
|------------|-----------------------------------|
| `fotoly`   | `~/.config/fotoly/config.toml`    |
| `pixelfox` | `~/.config/pixelfox/config.toml`  |

```toml
base_url = "https://fotoly.eu"
api_key  = "pxl_…"
```

Resolution order for API key and config path: **flag > env > file**. Default base URLs are `https://fotoly.eu` and `https://pixelfox.cc`.

### Environment variables

| Fotoly              | PixelFox               | Purpose              |
|---------------------|------------------------|----------------------|
| `FOTOLY_API_KEY`    | `PIXELFOX_API_KEY`     | API key              |
| `FOTOLY_BASE_URL`   | `PIXELFOX_BASE_URL`    | API base URL         |
| `FOTOLY_CONFIG`     | `PIXELFOX_CONFIG`      | Config file path     |

`NO_COLOR` disables ANSI colors when set.

## Global flags

Available on every command:

| Flag         | Description                          |
|--------------|--------------------------------------|
| `--json`     | Machine JSON on stdout; progress on stderr |
| `--config`   | Config file path                     |
| `--api-key`  | API key for this invocation          |

## Commands

Examples use `fotoly`; `pixelfox` is the same with its own config.

### Account

```bash
fotoly whoami          # alias: me
fotoly whoami --json
fotoly version
```

### Upload

```bash
fotoly upload photo.jpg
fotoly up photo1.png photo2.png --album 12
fotoly upload shot.jpg --processing original_only --no-wait
fotoly upload shot.jpg --copy      # force clipboard
fotoly upload shot.jpg --no-copy   # never copy
fotoly upload shot.jpg --json
```

Flags: `--album`, `--nsfw`, `--processing` (`default` or `original_only`), `--no-wait`, `--copy`, `--no-copy`.

### Images

```bash
fotoly images ls
fotoly images ls --limit 50 --album 12 --public
fotoly images ls --tag holiday --json
fotoly images get <uuid>
fotoly images status <uuid>
fotoly images edit <uuid> --title "Cat" --private --tag holiday --tag beach
fotoly images edit <uuid> --clear-tags
fotoly images delete <uuid…> --yes
```

`images ls` filters: `--limit`, `--cursor`, `--album`, `--public` / `--private` (mutually exclusive), `--nsfw` / `--sfw` (mutually exclusive), `--tag`.

`images status` prints whether processing is complete or failed, and the share URL once it exists. Use it after `upload --no-wait`.

`images edit` sends only the flags you set. `--title` and `--description` accept an empty string. `--public` / `--private` and `--nsfw` / `--sfw` are mutually exclusive pairs. Repeat `--tag` to replace the whole tag list. `--clear-tags` removes every tag and cannot be combined with `--tag`. At least one of these flags is required.

### Albums

```bash
fotoly albums ls
fotoly albums ls --json
```

Lists album id, title, image count, visibility, and share URL (useful as `--album` for upload). Album create/update/delete is not supported in v1.

## Agent skill

Assistants that read [Agent Skills](https://agentskills.io) can drive this CLI from the skill in the repo:

[`.agents/skills/fotoly-cli/SKILL.md`](.agents/skills/fotoly-cli/SKILL.md)

Grok, OpenCode, and Codex load that path when the working directory is this repository. The same file covers both binaries. It tells the assistant to pick `fotoly` or `pixelfox` for the brand you named, pass `--json`, and stay inside the commands above: setup, whoami, upload, image list/get/status/edit/delete, and album list. Album create/update/delete and moving an existing image onto an album are outside the CLI, so the skill stops there.

To use the skill from another project, copy the folder to `~/.agents/skills/fotoly-cli/`.

## Two binaries

`fotoly` and `pixelfox` never share config. Env prefixes do not leak across brands (`FOTOLY_*` does not affect `pixelfox`).

## Development

```bash
go test ./...
make build
./bin/fotoly version
./bin/pixelfox version
```

## Releases

Push a semver tag to publish binaries and a changelog. The tag looks like `v1.2.3`. A hyphen, as in `v1.2.3-rc.1`, marks a prerelease.

```bash
git tag v1.2.3
git push origin v1.2.3
```

The release workflow builds `fotoly` and `pixelfox` for Linux, macOS, and Windows on amd64 and arm64. Each archive contains both binaries. The release also gets `SHA256SUMS` and a changelog of the commits since the previous `v*` tag. The version inside the binary is the tag without the leading `v`.

## License

See [LICENSE](LICENSE).
