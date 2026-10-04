---
title: Requirements
description: What you need before installing beatstash, for a new server or an existing Navidrome.
prev:
  link: ../../
  label: Overview
---

Collect these before you run the installer. Each item links to the step that uses it.

## Server

- A Linux server, AMD64 or ARM64, that you reach over SSH.
- [Bash](https://www.gnu.org/software/bash/), [curl](https://curl.se/), and [Docker Engine](https://docs.docker.com/engine/install/) with the [Compose plugin](https://docs.docker.com/compose/install/linux/). Your user must be able to run `docker` without `sudo`.
- Disk space for your music, plus a little for the databases.

You don't need [Go](https://go.dev/doc/install) or a source checkout: the bot runs from a ready-made Docker image.

## Telegram

- A bot token from [@BotFather](https://t.me/BotFather).
- `api_id` and `api_hash` from [my.telegram.org](https://my.telegram.org). The stack runs a local [Telegram Bot API server](https://github.com/tdlib/telegram-bot-api) so the bot can handle files up to 2000 MiB; it needs these credentials.
- Your numeric Telegram user ID.

[Set up Telegram](./telegram.mdx) explains how to get each of them. The installer walks you through the same steps.

If Telegram is blocked where the server runs, you also need a proxy that reaches it. See [when Telegram is blocked](../administration/blocked-telegram.md).

## Navidrome

The deployment sample runs [Navidrome](https://www.navidrome.org/) **{{navidrome_version}}** and [Postgres](https://www.postgresql.org/) **{{postgres_version}}**.

To connect a Navidrome you already run, you also need:

- its administrator login and password;
- a version with multiple libraries; check yours against the one above before connecting an older release;
- a way to add a mount for a new music folder to it.

## A domain, for listening outside your network

To open Navidrome from your phone or send listening links, you need a domain or subdomain pointing at the server and ports 80 and 443 open. Uploads and sharing audio files in Telegram work without one. See [HTTPS and public access](./https.md).

## Next step

- [Install on a new server](./new-server.mdx) if you don't run Navidrome yet.
- [Connect an existing Navidrome](./existing-navidrome.mdx) if you do.
