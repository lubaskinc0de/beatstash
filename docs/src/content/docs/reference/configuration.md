---
title: Configuration reference
description: Every setting the bot reads at startup, with defaults and what it controls.
---

To apply a change, see [change the configuration](../administration/configuration.md). Settings not listed here, such as download concurrency, retry delays, and timeouts, are described in the commented [`config.example.toml`](https://github.com/lubaskinc0de/beatstash/blob/master/config.example.toml).

## Required

| Key | Where | What it is |
|---|---|---|
| `admins` | `config.toml` | Bot administrators, such as `telegram:123456789` |
| `library.music_dir` | `config.toml` | Writable folder for the bot's music, as the bot sees it |
| `navidrome.url` | `config.toml` | [Navidrome](https://www.navidrome.org/) address the bot can reach |
| `navidrome.user` | `config.toml` | Navidrome administrator login |
| `BOT_TOKEN` | `.env` | Telegram bot token |
| `SECRET_KEY` | `.env` | Base64 32-byte encryption key, from `openssl rand -base64 32` |
| `NAVIDROME_PASSWORD` | `.env` | Password of `navidrome.user` |
| `DB_DSN` | `.env` | [Postgres](https://www.postgresql.org/) connection string; the deployment's Compose file sets it |
| `POSTGRES_PASSWORD` | `.env` | Database password for the deployment sample |
| `TELEGRAM_API_ID`, `TELEGRAM_API_HASH` | `.env` | Credentials of the local Telegram Bot API |

## Addresses and paths

| Key | In the new-server deployment | What it is |
|---|---|---|
| `telegram.bot_api_url` | `http://telegram-bot-api:8081` | Local Bot API address; empty means Telegram's cloud API |
| `library.music_dir` | `/music` | The bot's music folder |
| `library.navidrome_music_dir` | `/music` | The same folder as Navidrome sees it |
| `navidrome.url` | `http://navidrome:4533` | Address the bot uses for Navidrome's API |
| `navidrome.public_url` | `https://music.example.com` | Address for players and listening links |

Keep existing libraries outside the [managed folders](./glossary.md#managed-folders) `shared`, `users`, and `.beatstash`. All three must be on one filesystem, and Navidrome must be able to read the files the bot creates.

Uploads work without `public_url`, but listening links need it. The bot turns links off when it is `localhost`, a private IP address, or a name that only resolves locally. See [HTTPS](../installation/https.md).

## Administrators and quotas

`admins` lists the bot's [administrators](./glossary.md#administrator). This role is separate from Navidrome's administrator. At startup, the listed accounts get the role, and accounts removed from the list lose it.

`admin_contact`, such as `@your_username`, is shown to strangers who open the bot, so they know whom to ask for an invite or more space.

`quota.default` is each participant's personal quota and `quota.shared` is the shared library's. Use values such as `10GB` or `500MB`; an empty string means unlimited. Values set in **Admin > Quotas** override these, and **Admin > Users** sets limits for individual people.

## Telegram

| Key | Default | What it does |
|---|---|---|
| `telegram.storage_chat_id` | `0` | Private channel for preparing audio files; see [storage chat](../administration/storage-chat.md) |
| `telegram.fill_storage_chat` | `true` | Upload to the storage chat in the background instead of on request |

## Intervals

| Key | Default | What it does |
|---|---|---|
| `navidrome.attach_interval` | `1h` | Refresh records of existing libraries; `0` turns existing libraries off |
| `navidrome.access_ttl` | `30s` | How long the bot trusts what Navidrome said an account can see; `0` asks every time |
| `library.reconcile_interval` | `1h` | Pick up files changed by hand in the managed folders |
| `library.enrich_interval` | `1h` | Look up missing genres and labels again, retrying services that didn't answer |
| `navidrome.song_interval` | `1m` | Match new tracks to Navidrome songs; `0` matches only on request |
| `zvuk.sync_interval` | `6h` | Check imported Zvuk collections for changes |

## Invites and listening links

| Key | Default | What it does |
|---|---|---|
| `invites.ttl` | `168h` | How long an invite link works |
| `navidrome.listen_link_ttl` | `720h` | How long a listening link works |
| `navidrome.listen_link_downloadable` | `true` | Let listening links download files |

Raising download concurrency for a streaming service makes the account more likely to be throttled. By default the bot pauses between downloads and downloads one track at a time per Zvuk account.
