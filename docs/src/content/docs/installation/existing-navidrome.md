---
title: Connect an existing Navidrome server
description: Add beatstash without moving your existing music collection.
---

Connect beatstash to the [Navidrome](https://www.navidrome.org/) server you already use. You can then find your music in Telegram, send tracks to friends, and add more music by forwarding audio files to the bot or importing from a supported music service.

## What happens to your existing music

Your collection stays in its current folders, and you can keep listening through Navidrome and your usual apps. You do not need to upload it again. beatstash adds your existing libraries and tracks to its own catalog so you can find them through the bot. It does not move or change the original audio files.

After linking your Navidrome account, you can search the libraries you have access to, request a track's audio file, or share music in a Telegram chat. For example, if a friend asks about an album you already own, you can find a song from it without leaving the conversation. Other participants can only browse existing libraries their Navidrome accounts can access.

New uploads and streaming imports go into your personal library. You can choose which tracks to add to the shared library for everyone on the server. If an imported song matches one already in an existing library you can access, beatstash uses that track instead of downloading another copy.

Your existing collection does not use up your beatstash storage quota. Adding one of its tracks to the shared library creates a separate copy, which counts toward the shared library's quota. See [libraries and storage limits](../using/libraries.md).

## Requirements

- Administrative access to your Navidrome server. A regular listening account cannot set up this integration.
- A Navidrome instance with multi-library support. The project's tests use version **0.64.1**; a minimum compatible version has not been established.
- A new writable music directory for beatstash, mounted into Navidrome too.
- [Postgres](https://www.postgresql.org/) and the local [Telegram Bot API server](https://github.com/tdlib/telegram-bot-api) from the deployment sample.

## Connect with the guided setup

Install [curl](https://curl.se/) and [Docker Engine](https://docs.docker.com/engine/install/) with the [Compose plugin](https://docs.docker.com/compose/install/linux/) on the Linux server where beatstash will run. Download the installer for **{{release_tag}}**:

```sh
curl -fL https://github.com/lubaskinc0de/beatstash/releases/download/{{release_tag}}/install.sh -o install.sh
curl -fL https://github.com/lubaskinc0de/beatstash/releases/download/{{release_tag}}/install.sh.sha256 -o install.sh.sha256
sha256sum -c install.sh.sha256
bash install.sh
```

Choose `existing`. The installer asks for your Navidrome address and administrator account, guides you through Telegram setup, and configures the bot's services. It uses your existing Navidrome instead of starting another one.

You will need to add one folder for new uploads to your Navidrome setup. The installer shows the exact mount for your chosen installation directory and waits while you apply it. Keep your current music folders and libraries. For a native installation, use the host folder path and make sure the Navidrome user can read it and its parent directories. See [mounting the new directory](#2-mount-the-new-music-directory) for details.

Rerun the installer in the same directory if you need to finish setup. Press Enter to keep saved values. After it starts, [link your existing account](#4-start-and-link-accounts) and [verify the connection](#5-verify-the-connection).

## Manual setup

## 1. Prepare beatstash

Download the installation archive for your chosen release and prepare `deploy/.env`, `deploy/config.toml`, and Telegram as described in [steps 1 to 3 of the new-server guide](./new-server.md). The bot runs from the ready-made [Docker](https://docs.docker.com/) image. Do not start the sample's Navidrome service or create a new Navidrome administrator.

Work from the `deploy` directory. In `.env`, set `COMPOSE_PROFILES=""`, so the sample's own `navidrome` service does not start. `postgres`, `telegram-bot-api`, and `bot` always run.

## 2. Mount the new music directory

For example, if you unpacked the archive into `/srv/beatstash`, the bot writes to `/srv/beatstash/deploy/music` on the host. Add that directory to your existing Navidrome container:

```yaml
volumes:
  # Keep your existing data and music mounts here.
  - /srv/beatstash/deploy/music:/beatstash-music:ro
```

Create its `shared` and `users` subdirectories before restarting Navidrome. Keep all folders managed by the bot on the same filesystem. Navidrome needs read access; the bot needs write access.

Keep the existing collection as its own library. A library pointing at `/beatstash-music` would include every participant's personal folder, so beatstash ignores it and removes it from linked regular accounts. The bot creates libraries for the individual subdirectories itself.

If Navidrome runs directly on the host, use its host path instead of `/beatstash-music` in the configuration below.

## 3. Set the addresses and administrator account

Edit the existing sections in `deploy/config.toml`:

```toml
[library]
music_dir = "/music"
navidrome_music_dir = "/beatstash-music"

[navidrome]
url = "https://music.example.com"
public_url = "https://music.example.com"
user = "admin"
```

`music_dir` is the path inside the bot. `navidrome_music_dir` is the same directory as seen by Navidrome. `url` must be reachable from the bot container; `localhost` would point to the bot itself.

Using the existing HTTPS address is one option. For an internal Docker address, attach both containers to a shared Docker network and use Navidrome's service name. Keep `public_url` as the address your listeners open.

Set `NAVIDROME_PASSWORD` in `deploy/.env` to the password of the administrator named by `navidrome.user`. Enable sharing in Navidrome if you want [public listening links](../administration/https.md).

## 4. Start and link accounts

```sh
cd /srv/beatstash/deploy
docker compose pull
docker compose up -d
docker compose logs --tail=100 bot
```

Open the bot with `/start`. In **Settings > Navidrome account**, choose **Link** and send your existing login and password. Invited participants can choose **I already have an account** during registration.

Linking a regular account preserves access to separate existing libraries, adds its personal library and the shared library, and removes access to other participants' personal libraries and libraries containing the bot's managed folders. A linked Navidrome administrator continues to see every library.

The bot also changes Navidrome's default libraries for newly created accounts to the shared library. Existing-library access for a new participant can be granted in Navidrome by its administrator.

## 5. Verify the connection

Search **Music > Mine** for a track from an existing library. You should be able to request its file or share it. New uploads should appear in your new personal library; your original files should remain in place.

Navidrome must scan changes before the bot can register them. beatstash refreshes attached libraries at startup and every hour by default. Access changes may take up to the configured `access_ttl`, 30 seconds by default.

Existing attached libraries do not count toward beatstash quotas. Publishing one of their tracks creates a copy in the shared library, where the shared quota applies. An import skips a matching track already present in an attached library you can access.

Use [the participant guide](../using/getting-started.md) for friends and [troubleshooting](../administration/troubleshooting.md) if a library is missing.
