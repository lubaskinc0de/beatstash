---
title: Change the configuration
description: Where the settings live and how to apply a change without losing it.
---

Settings live in your installation's `deploy` folder, `~/beatstash/deploy` if you kept the installer's default:

- `config.toml` holds the bot's settings: addresses, paths, administrators, quotas, intervals;
- `.env` holds secrets and the choice of services.

Every key is described in the [configuration reference](../reference/configuration.md), and the commented template is [`config.example.toml`](https://github.com/lubaskinc0de/beatstash/blob/master/config.example.toml).

## Change a setting

The bot reads both files only at startup. Edit, then recreate its container:

```sh
cd ~/beatstash/deploy
nano config.toml        # or any editor
docker compose up -d --force-recreate bot
docker compose logs --tail=50 bot
```

:::caution[A restart is not enough]
[Docker](https://docs.docker.com/) mounts `config.toml` as a single file. Many editors save a new file in its place, and a running container keeps seeing the old one. `--force-recreate` picks up the new file.
:::

Look for `bot_started` in the log. If the bot exits right away, the log names the setting it rejected. It also rejects keys it doesn't know, so compare spelling with the template.

Other changes need other services restarted:

| You changed | Run |
|---|---|
| `config.toml`, or `BOT_TOKEN`, `SECRET_KEY`, `NAVIDROME_PASSWORD` in `.env` | `docker compose up -d --force-recreate bot` |
| `TELEGRAM_API_ID`, `TELEGRAM_API_HASH` | `docker compose up -d --force-recreate telegram-bot-api bot` |
| `COMPOSE_PROFILES`, `COMPOSE_FILE`, `TELEGRAM_PROXY`, or `compose.yml` | `docker compose up -d` |

Running the installer again in the same directory also works: it changes the answers it asks about and restarts what needs it.

## Secrets go in `.env`

Keep passwords and tokens out of `config.toml`. The bot doesn't read `.env` itself; [Docker Compose](https://docs.docker.com/compose/) passes it in. The application looks for `config.toml` in its working directory, or wherever `CONFIG_FILE` points; the deployment mounts `deploy/config.toml` as `/app/config.toml`.

:::danger[Don't replace `SECRET_KEY`]
The bot encrypts saved Navidrome passwords and streaming tokens with it. A new key can't decrypt them. Keep the original across restarts and upgrades.
:::
