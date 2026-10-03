---
title: Configuration
description: Set addresses, library paths, administrator access, quotas, and import intervals.
---

Non-secret settings belong in TOML. Passwords and tokens for startup belong in environment variables. The full commented template is [`config.example.toml`](https://github.com/lubaskinc0de/beatstash/blob/master/config.example.toml).

The application reads `config.toml` in its working directory unless `CONFIG_FILE` points elsewhere. It does not load `.env` itself: Docker Compose or `just` loads it. The deployment sample mounts `deploy/config.toml` as `/app/config.toml`.

## Required startup values

| Value | Location | Purpose |
|---|---|---|
| `admins` | TOML | Administrator identities such as `telegram:123456789` |
| `library.music_dir` | TOML | Writable managed music directory as seen by the bot |
| `navidrome.url` | TOML | Navidrome address reachable by the bot |
| `navidrome.user` | TOML | Navidrome administrator login |
| `BOT_TOKEN` | Environment | Telegram bot credential |
| `DB_DSN` | Environment | Postgres connection string; set by the deployment Compose file |
| `SECRET_KEY` | Environment | Base64-encoded 32-byte encryption key |
| `NAVIDROME_PASSWORD` | Environment | Password for `navidrome.user` |
| `POSTGRES_PASSWORD` | Deployment `.env` | Database password used by the deployment sample |
| `TELEGRAM_API_ID`, `TELEGRAM_API_HASH` | Deployment `.env` | Credentials used by the local Telegram API service |

Generate the encryption key with `openssl rand -base64 32`. Keep it across restarts and upgrades. Replacing it makes existing encrypted account credentials unreadable.

## Addresses and paths

| Setting | Example in the new-server deployment | Meaning |
|---|---|---|
| `telegram.bot_api_url` | `http://telegram-bot-api:8081` | Internal local Bot API address; empty uses the cloud API |
| `library.music_dir` | `/music` | Bot's mount for new files |
| `library.navidrome_music_dir` | `/music` | The same mount as seen by Navidrome |
| `navidrome.url` | `http://navidrome:4533` | Bot-to-Navidrome API address |
| `navidrome.public_url` | `https://music.example.com` | Address for players and public listening links |

Keep existing libraries separate from `shared`, `users`, and `.beatstash`. All three managed directories must share a filesystem. Files created by the bot must be readable by Navidrome.

`public_url` is optional for uploads, but required for public links. Localhost, private IP addresses, and local-only hostnames disable listening links in the bot. See [HTTPS setup](./https.md).

## Administrator access and limits

`admins` controls beatstash administrators. It is separate from Navidrome's administrator role. At startup, configured identities receive the bot's administrator role; identities removed from the list lose it.

Set `admin_contact` to a contact such as `@your_username` if strangers and participants should know where to request access or more quota.

`quota.default` and `quota.shared` take values such as `10GB` or `500MB`; an empty string means unlimited. Values set through **Admin > Quotas** override configuration. Individual overrides are managed through **Admin > Users**.

## Intervals and public links

| Setting | Default | Purpose |
|---|---|---|
| `navidrome.attach_interval` | `1h` | Refresh records for existing libraries; `0` disables attached libraries |
| `navidrome.access_ttl` | `30s` | Cache a user's existing-library access; `0` checks every time |
| `library.reconcile_interval` | `1h` | Reconcile managed library records with on-disk changes |
| `navidrome.song_interval` | `1m` | Resolve newly indexed songs; `0` leaves lookup to individual requests |
| `zvuk.sync_interval` | `6h` | Poll imported Zvuk collections for changes |
| `invites.ttl` | `168h` | Single-use invite lifetime |
| `navidrome.listen_link_ttl` | `720h` | Public listening link lifetime |
| `navidrome.listen_link_downloadable` | `true` | Allow downloads through listening links |

Use the commented template for ingestion concurrency, retry delays, and timeouts. More provider concurrency can increase account throttling; the defaults use pauses and one download per Zvuk account.

Restart the bot after TOML or startup environment changes:

```sh
docker compose up -d --force-recreate bot
```

Run this from your installation's `deploy` directory. Read [Telegram setup](../installation/telegram.md) for storage-chat settings and [updates](./updates.md) before changing service versions.
