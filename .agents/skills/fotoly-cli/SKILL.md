---
name: fotoly-cli
description: Use when uploading, listing, editing, or deleting images, checking processing status, listing albums, or checking the account on Fotoly or PixelFox with the fotoly or pixelfox CLI. Triggers include fotoly, pixelfox, fotoly-cli, fotoly.eu, pixelfox.cc, API key, Bild hochladen, Alben auflisten, and /fotoly-cli.
user-invocable: true
---

# fotoly-cli

Use the official CLI. `fotoly` and `pixelfox` share one command set and never share config. Do not call the HTTP API.

## Binary

| Brand | Binary | Host | Config | Env prefix |
|---|---|---|---|---|
| Fotoly | `fotoly` | `https://fotoly.eu` | `~/.config/fotoly/config.toml` | `FOTOLY_` |
| PixelFox | `pixelfox` | `https://pixelfox.cc` | `~/.config/pixelfox/config.toml` | `PIXELFOX_` |

Env names are `<PREFIX>API_KEY`, `<PREFIX>BASE_URL`, and `<PREFIX>CONFIG`. A Fotoly variable does not affect `pixelfox`, and the reverse.

Use the binary for the brand the user named. If they name none, use the only binary that is installed or the only config file that exists. If both brands are set up, ask which brand.

If `command -v <binary>` fails:

```bash
go install github.com/ManuelReschke/fotoly-cli/cmd/<binary>@latest
```

## Every call

Add `--json` to account, album, and image commands.

- stdout is one JSON value, the result
- stderr is progress, warnings, and on failure `{"error","message"}`
- exit `0` succeeded, `1` failed, `2` bad usage

Do not pass `--api-key` after setup. Do not print the key or the config file.

Check the session with `<binary> whoami --json`.

When the message is `No API key. Run '<binary> setup'.`, ask the user for a key from `https://fotoly.eu/user/settings` or `https://pixelfox.cc/user/settings`. Keys start with `pxl_`. Run this once:

```bash
<binary> setup --api-key pxl_…
```

Setup prints `Logged in as <user> (<plan>) on <url>` and does not emit JSON. Then run `whoami --json`.

When the message is `Invalid API key. Run '<binary> setup'.`, stop and ask for a new key.

## Commands

`whoami` (alias `me`) returns `username`, `email`, `plan`, `stats.images.count`, `stats.images.storage_used_bytes`, `stats.albums.count`, and `limits.max_upload_bytes`.

`upload <files...>` (alias `up`). Always pass `--no-copy`. Optional flags: `--album <id>` from `albums ls`, `--nsfw`, `--processing default|original_only`, `--no-wait`. stdout is an array of `{file, ok, image_uuid, url, duplicate, error}`. `url` is absolute when `ok` is true. The command waits until processing finishes. `--no-wait` returns before that; use `images status` until `complete` is true, then `images get`. Exit `1` if any file failed; still read the stdout array. This stderr line means the image was stored but not added to the album: `album <id> was not bound; uploading without album`.

`images ls` filters: `--limit` (default 25, maximum 100), `--cursor`, `--album`, `--public` or `--private`, `--nsfw` or `--sfw`, `--tag`. Each pair is mutually exclusive. stdout is `{items, has_more, next_cursor}`. An item has `image_uuid`, `title`, `description`, `file_name`, `file_size`, `is_public`, `is_nsfw`, `view_url`. The list has no tags. When `has_more` is true and the user wants the rest, repeat with `--cursor` set to `next_cursor`.

`images get <uuid>` returns `image_uuid`, `url`, `view_url`, `is_nsfw`, `tags`, and `available_variants`.

`images status <uuid>` returns `complete`, `failed`, and `view_url`. `view_url` is null until processing finishes.

`images edit <uuid>` updates metadata. Send only the flags that should change: `--title`, `--description` (empty string clears), `--public` or `--private`, `--nsfw` or `--sfw`, repeatable `--tag` (replaces the whole list), or `--clear-tags`. Each pair is mutually exclusive. `--tag` with `--clear-tags` is a usage error. At least one flag is required. stdout is the updated image, including `tags`.

`images delete <uuid...> --yes` runs only after the user has explicitly asked to delete those images. Without `--yes`, a non-interactive run deletes nothing and exits `1` with `use --yes to delete`. stdout is an array of `{image_uuid, status, message, error}`.

`albums ls` returns `{albums: [{id, title, description, is_public, is_nsfw, image_count, view_url, share_link}]}`.

A relative `view_url` is a path. Prefix the brand host. An upload result `url` is already absolute.

## No command exists

Say so and stop. Do not invent a flag and do not call the HTTP API.

- create, edit, or delete an album
- add or remove an existing image on an album

`setup --reset` deletes that brand's config. Run it only when the user asks to log out.
