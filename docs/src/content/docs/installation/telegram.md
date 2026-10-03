---
title: Set up Telegram
description: Create your bot, configure inline mode, and connect the local Bot API server.
---

## Create a bot

1. Open [@BotFather](https://t.me/BotFather) in Telegram.
2. Use `/newbot`, choose a name and username, and save the token in `BOT_TOKEN` in your installation's `.env`.
3. Use `/setinline`, select your bot, and set a prompt such as `Search your music`.
4. Use `/setinlinefeedback`, select your bot, and choose **Enabled**, not a fraction such as 1/100. beatstash uses chosen-result updates to prepare tracks and listening links after selection; with a fraction, most picked tracks stay at ⏳.

Once beatstash is running, open your bot and send `/start`. From there, you can invite friends, connect a music service, or browse your collection.

## Find your numeric user ID

Open [@Get_myidrobot](https://t.me/Get_myidrobot) in Telegram and press **Start**. Copy your numeric user ID from its reply.

In `deploy/config.toml`, replace the example ID in `admins = ["telegram:123456789"]` with your own. This tells beatstash who can manage the server and invite friends. Use the number, not your Telegram username.

## Run the local Bot API

Sign in at [my.telegram.org](https://my.telegram.org), open **API development tools**, and create an application to obtain `api_id` and `api_hash`. Put them in `TELEGRAM_API_ID` and `TELEGRAM_API_HASH` in `.env`.

The interactive installer handles the cloud logout for you. For manual setup, stop any other instance of this bot and log it out before the first local start. Paste the token from BotFather when prompted; input is hidden:

```sh
read -r -s -p 'Bot token: ' bot_token
printf '\n'
printf 'url = "https://api.telegram.org/bot%s/logOut"\n' "$bot_token" | \
  curl --config - --fail --silent --request POST
unset bot_token
```

Expect `"ok":true`. Keep `.env` as a Compose configuration file; sourcing it in a shell can change values containing special characters. Then set `telegram.bot_api_url` to `http://telegram-bot-api:8081` and start the local service with the bot.

The bot and local API must share the `telegram_bot_api_data` volume at the same path, `/var/lib/telegram-bot-api`. The local API returns local file paths; the bot needs access to them. The deployment sample supplies this mount.

Telegram documents the switch and local server capabilities in [Using a Local Bot API Server](https://core.telegram.org/bots/api#using-a-local-bot-api-server).

## When Telegram is blocked on the server

The local Bot API connects to Telegram's data centers directly. Where Telegram is blocked, in the country or by the hosting provider, it cannot sign in, and the bot fails to start with `getMe` timeouts in its log. Check from the server:

```sh
curl -m 10 -sS -o /dev/null https://api.telegram.org && echo reachable
```

The guided setup checks this itself and, only when Telegram is unreachable, asks for a proxy that reaches it. The step is optional; without it, the bot starts once the server can reach Telegram.

The proxy is given as `http://host:port` or `socks5://host:port`. Usually it is a VPN client running on the same server, such as xray, v2ray or sing-box: take the protocol and port of its HTTP or SOCKS inbound from the `inbounds` section of its configuration. Containers address the server as `host.docker.internal`, so that inbound must listen on the Docker gateway `172.17.0.1`, not only on `127.0.0.1`. For example, with an xray HTTP inbound on port 10809:

```json
{ "protocol": "http", "listen": "172.17.0.1", "port": 10809 }
```

Check it with `curl -x http://172.17.0.1:10809 https://api.telegram.org`, then enter `http://host.docker.internal:10809`.

The Bot API's own `--proxy` option covers webhooks only, so the deployment routes the whole Bot API container through the proxy instead: `compose.telegram-proxy.yml` adds a small `telegram-proxy` container (tun2socks) whose network the Bot API shares. Other services keep connecting directly. To enable it by hand, add to `deploy/.env`:

```dotenv
COMPOSE_FILE="compose.yml:compose.telegram-proxy.yml"
TELEGRAM_PROXY="http://host.docker.internal:10809"
```

and run `docker compose up -d`. The server needs `/dev/net/tun`, which most virtual servers have. The installer's own logout from the cloud API goes through the same proxy.

## Optional storage chat

A storage chat lets beatstash prepare Telegram audio files for music imported from elsewhere or read from an existing Navidrome library.

1. Create a private channel and add your bot as an administrator with permission to post.
2. Obtain the channel's numeric chat ID from a `channel_post` update while setting up the bot. Private channel IDs usually start with `-100`; use the actual ID from Telegram.
3. Set `telegram.storage_chat_id` to that ID in `deploy/config.toml` and [apply the change](../administration/configuration.md#change-a-setting).

With `fill_storage_chat = true`, the bot prepares files in the background. With it disabled, files are prepared as users request them. Leaving the chat ID at `0` skips background storage; some inline results depend on public listening links or preparing a file after selection.

## Cloud API alternative

Leave `telegram.bot_api_url` empty to use Telegram's cloud API, and remove the local API dependency from your Compose setup. Incoming file downloads through the cloud API have a 20 MB limit. The current bot caps outgoing files at 50 MiB with the cloud API and 2000 MiB with a local API configured; Telegram may impose additional limits.

For switching back, stop the bot and log it out through the local API first. Follow Telegram's current migration rules; after a cloud logout, returning to the cloud API has a waiting period. Do not poll both APIs with the same bot simultaneously.
