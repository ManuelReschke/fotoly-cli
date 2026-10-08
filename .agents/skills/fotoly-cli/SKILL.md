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

If `command -v <binary>` fails, install both binaries. On macOS or Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/ManuelReschke/fotoly-cli/main/install.sh | sh
```

On Windows:

```powershell
powershell -ExecutionPolicy ByPass -c "irm https://raw.githubusercontent.com/ManuelReschke/fotoly-cli/main/install.ps1 | iex"
```

The installer puts `fotoly` and `pixelfox` in `~/.local/bin`. If that directory is not on `PATH`, use the path the installer prints.

## Every call

Add `--json` to account, album, image, and notification commands.

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

`images like <uuid>` and `images unlike <uuid>` return `{image_uuid, liked, like_count}`. The image must be owned by the account or public.

`images comments ls <uuid>` returns `{image_uuid, total_count, comments}`. The server returns only the newest 30 top-level comments and their replies. Older roots are omitted and there is no cursor. `total_count` still counts every comment that is not deleted, including ones outside this page. When older roots were omitted, stderr says `older comments omitted` and `comments` is not the full thread. Each comment has `id`, `username`, `content`, `deleted`, `can_delete`, and `replies`. A deleted comment has empty `content`.

`images comments add <uuid> --content <text>` creates a comment. Optional `--reply-to <id>` replies to a top-level comment on that image. `--content` is required, trimmed, and at most 2000 characters.

`images comments delete <id...> --yes` deletes comments the account wrote. Same `--yes` rule as `images delete`. stdout is an array of `{id, status, error}`.

`albums ls` returns `{albums: [{id, title, description, is_public, is_nsfw, image_count, view_url, share_link}]}`.

`albums create --title <title>` creates an album. Optional: `--description`, `--public` or `--private`, `--nsfw` or `--sfw`, `--password`, `--sort desc|asc`. Each pair is mutually exclusive. Omitted visibility stays private. stdout is the album, including `image_sort_order` and `has_share_password` when the server sends them. The password itself is never returned.

`albums edit <id>` updates an album. Send only the flags that should change: `--title`, `--description` (empty allowed), `--public` or `--private`, `--nsfw` or `--sfw`, `--password` (empty clears it), `--sort desc|asc`. At least one flag is required. `--title` cannot be empty.

`albums delete <id...> --yes` deletes albums. Images stay in the library. Same `--yes` rule as `images delete`. The prompt is `Delete N album(s)?`. stdout is an array of `{id, status, error}`.

`albums images ls <id>` lists images assigned to that album. stdout is `{album, images}`. Each image includes `tags` (an empty array when there are none), `view_count`, `download_count`, `processing_profile`, and `last_viewed_at` (null when the image has not been viewed).

`albums images add <id> <uuid...>` assigns owned images. stdout is `{added, already_assigned, image_count}`.

`albums images delete <id> <uuid...> --yes` removes the assignment. The image stays in the library. Without `--yes`, a non-interactive run exits `1` with `use --yes to remove`. The prompt is `Remove N image(s) from the album?`.

`albums cover <id>` sets the cover with `--image <uuid>` or clears it with `--clear`. Exactly one of those flags. The image must already be in the album. `--json` includes `cover_image_uuid`, null when cleared.

`albums members ls <id>` returns `{members: [{user_id, username, role, origin, created_at}]}`. Email addresses are not included.

`albums members add <id>` invites one viewer. Set exactly one of `--user-id <id>` or `--username <name>`. When the username matches several accounts, the command fails and the message lists `name (id)` candidates. Invite again with `--user-id`. stdout is the member plus `already_member`.

`albums members delete <id> <user-id...> --yes` removes members. Without `--yes`, a non-interactive run exits `1` with `use --yes to remove`.

`albums categories ls` returns `{categories: [{id, name, slug, scope, album_count}]}`. `scope` is `private` or `public`.

`albums categories create --name <name>` creates a private category. Names may contain letters, digits, and hyphens.

`albums categories set <id> --private <ids> --public <ids>` replaces both lists. Both flags are required. An empty value clears that side. Ids are comma-separated. stdout is the category collection now on the album.

`notifications ls` returns `{unread_count, items, has_more, next_cursor}`. Optional `--limit` (default 30) and `--cursor`. The server accepts a limit from 1 to 100. An item has `id`, `type`, `title`, `body`, `target_url`, `is_read`, `actor_name`, `actor_count`, `event_count`, and `last_event_at`. When `has_more` is true, repeat with `--cursor` set to `next_cursor`.

A relative `view_url` or `target_url` is a path. Prefix the brand host. An upload result `url` is already absolute.

## No command exists

Say so and stop. Do not invent a flag and do not call the HTTP API.

- `/auth/*` session routes, including register and login
- `GET /ping`
- upload processing profile `custom` with an explicit derivative list

`setup --reset` deletes that brand's config. Run it only when the user asks to log out.
