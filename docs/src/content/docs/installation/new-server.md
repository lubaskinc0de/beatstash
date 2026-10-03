---
title: Install on a new server
description: Set up Navidrome, Postgres, a local Telegram Bot API server, and beatstash with Docker Compose.
---

This guide installs the whole stack on a Linux server. If you already run Navidrome, use [the connection guide](./existing-navidrome.md) instead.

## Before you start

You need SSH access, Bash 4 or later, curl, jq, OpenSSL, and Docker Engine with the Compose plugin. Install Docker using its [Linux installation guide](https://docs.docker.com/engine/install/). You also need enough disk space for your music and databases. The ready-made beatstash image supports Linux AMD64 and ARM64; no Go compiler or source checkout is needed.

The sample uses Navidrome **0.64.1**, the version used by the project's end-to-end tests. Other versions need checking before use. It includes a local Telegram Bot API server to handle larger files.

## Install with the guided setup

The latest stable release is **{{release_tag}}**. Download its installer on your server:

```sh
curl -fL https://github.com/lubaskinc0de/beatstash/releases/download/{{release_tag}}/install.sh -o install.sh
curl -fL https://github.com/lubaskinc0de/beatstash/releases/download/{{release_tag}}/install.sh.sha256 -o install.sh.sha256
sha256sum -c install.sh.sha256
bash install.sh
```

Choose `new` when asked about Navidrome. The installer walks you through creating a Telegram bot, enabling sharing in chats, and getting the Telegram credentials. It generates the database password and encryption key, saves your settings, and starts Navidrome so you can create its first administrator. Once that account is ready, it starts the bot.

Passwords and tokens are hidden while you enter them. If you stop partway through, run the installer again and choose the same directory. Press Enter at a filled prompt to keep its saved value. It keeps your Compose file, configuration, and generated secrets. Use [the update guide](../administration/updates.md) to change versions later.

On a remote server, the installer shows an SSH tunnel command so you can open Navidrome from your own computer. After setup, continue with [checking the installation](#6-check-the-installation) and [HTTPS](../administration/https.md).

## Manual setup

If you prefer to configure the stack yourself, follow the steps below.

## 1. Download the installation files

The latest stable release is **{{release_tag}}**. The commands below download its Compose and configuration templates, with the matching Docker image version already selected. Release notes are on [GitHub Releases](https://github.com/lubaskinc0de/beatstash/releases).

```sh
release_tag="{{release_tag}}"
mkdir -p beatstash
cd beatstash
curl --fail --location --remote-name \
  "https://github.com/lubaskinc0de/beatstash/releases/download/$release_tag/beatstash-$release_tag-deploy.tar.gz"
curl --fail --location --remote-name \
  "https://github.com/lubaskinc0de/beatstash/releases/download/$release_tag/beatstash-$release_tag-deploy.tar.gz.sha256"
sha256sum -c "beatstash-$release_tag-deploy.tar.gz.sha256"
tar -xzf "beatstash-$release_tag-deploy.tar.gz"
cp deploy/.env.example deploy/.env
cp deploy/config.example.toml deploy/config.toml
mkdir -p deploy/music/shared deploy/music/users deploy/data/navidrome
chmod 600 deploy/.env deploy/config.toml
cd deploy
```

Stop if the checksum does not match. The installation uses `deploy/compose.yml` and pulls `ghcr.io/lubaskinc0de/beatstash` at the version in `BEATSTASH_VERSION`. Keep this exact version rather than using `latest`, so a restart does not unexpectedly select a different release.

Keep `music/shared`, `music/users`, and the bot's `music/.beatstash` directory on the same filesystem: the bot uses hardlinks and renames between them.

## 2. Set up Telegram and secrets

Follow [Telegram setup](./telegram.md) to create the bot, enable inline mode, obtain your numeric Telegram user ID, and get the local Bot API credentials.

Edit `.env` and set:

```dotenv
BOT_TOKEN="<token from BotFather>"
SECRET_KEY="<output of openssl rand -base64 32>"
NAVIDROME_PASSWORD="<password you will use for the Navidrome administrator>"
TELEGRAM_API_ID="<your api_id>"
TELEGRAM_API_HASH="<your api_hash>"
POSTGRES_PASSWORD="<output of openssl rand -hex 32>"
```

Generate the two random values separately:

```sh
openssl rand -base64 32
openssl rand -hex 32
```

Keep the `BEATSTASH_VERSION` supplied by the archive, without the leading `v`. Keep the original `SECRET_KEY`. Losing it prevents the bot from decrypting saved passwords and streaming tokens. The deployment Compose file sets `DB_DSN` itself.

Switch the bot from Telegram's cloud API to the local API as described in the Telegram guide before starting it here.

## 3. Configure the bot

Edit the matching fields in `config.toml`, keeping the remaining settings from the example:

```toml
admins = ["telegram:123456789"]
admin_contact = "@your_telegram_username"

[telegram]
bot_api_url = "http://telegram-bot-api:8081"

[library]
music_dir = "/music"
navidrome_music_dir = "/music"

[navidrome]
url = "http://navidrome:4533"
public_url = ""
user = "admin"
```

Replace `123456789` with your own numeric ID. These are edits to existing sections, not extra copies of the sections to append. Internal addresses such as `http://navidrome:4533` are only reachable inside the Compose network.

Leave `public_url` empty until you have set up [HTTPS](../administration/https.md). Music uploads work without public listening links.

## 4. Create the first Navidrome administrator

```sh
docker compose up -d postgres navidrome
docker compose ps
```

Navidrome listens on the server's loopback interface. On your own computer, open an SSH tunnel:

```sh
ssh -L 4533:127.0.0.1:4533 your_user@your_server
```

Open `http://localhost:4533`, create the first administrator with login `admin`, and use the password you put in `NAVIDROME_PASSWORD`. If you choose a different login, update `navidrome.user` too.

The bot does not create the first Navidrome administrator. Finish this step before starting it.

## 5. Start beatstash

```sh
docker compose pull telegram-bot-api bot
docker compose up -d telegram-bot-api bot
docker compose logs --tail=100 bot
docker compose ps
```

Open your bot in Telegram and send `/start`. You should see the **Admin** and **Invite** buttons. In **Settings > Navidrome account**, link your account using **Link**, then send `login password` as requested. The bot deletes that input message.

Your linked administrator account sees every Navidrome library. For everyday use with private-library access, use a separate non-administrator account.

## 6. Check the installation

1. Send the bot a tagged audio file. Wait for the thumbs-up reaction.
2. Open Navidrome and wait for it to scan the track. Confirm that you can play it.
3. Create an invite and have another participant join with a new account.
4. Confirm that the participant sees their personal library and the shared library, without seeing your personal library.
5. Share the track in **Music > Mine**. Confirm that the participant can find it in **Music > Shared**.

Next, set up [HTTPS](../administration/https.md) for access outside the SSH tunnel. Review [configuration](../administration/configuration.md) before increasing quotas or enabling public links.
