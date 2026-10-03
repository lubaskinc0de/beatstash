---
title: HTTPS and public access
description: Expose Navidrome through Caddy and enable public listening links.
---

The deployment sample exposes Navidrome only on `127.0.0.1:4533`. This guide adds a host-installed Caddy reverse proxy. beatstash uses Telegram long polling and does not need an inbound bot webhook.

## Before you start

You need a domain pointing to your server and inbound ports **80 and 443** open. Remove an AAAA record if your server does not actually accept IPv6 connections. Keep Postgres and the local Telegram Bot API off the public network.

Install Caddy using its [official instructions](https://caddyserver.com/docs/install). If a proxy already runs on the server, adapt that setup instead of binding a second proxy to the same ports.

## Configure Caddy

Add this site to `/etc/caddy/Caddyfile`, replacing the example domain:

```text
music.example.com {
    reverse_proxy 127.0.0.1:4533
}
```

Validate and reload:

```sh
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

Caddy obtains and renews HTTPS certificates when the domain and network meet its [automatic HTTPS requirements](https://caddyserver.com/docs/automatic-https). This example assumes Caddy runs on the host; a proxy inside Docker needs a shared network and the Navidrome service address instead of host loopback.

## Set the public address

Edit the existing Navidrome section in `deploy/config.toml`:

```toml
[navidrome]
url = "http://navidrome:4533"
public_url = "https://music.example.com"
user = "admin"
```

Keep your actual internal URL and login. Restart the bot from `deploy`:

```sh
docker compose up -d --force-recreate bot
```

The new-server sample already sets `ND_ENABLESHARING=true`. For an existing Navidrome, enable sharing in its own configuration. A reverse proxy must allow `/share` and its resources; protecting that route with an extra proxy login prevents anonymous listening.

## Verify access

1. Open `https://music.example.com` from outside your server's network and sign in.
2. Add that address to your listening client and play a track.
3. Request an album's **Listening link** in the bot.
4. Open the link in a private browser window without a Navidrome session.
5. Check whether downloading is available according to `listen_link_downloadable`.

Links work for anyone who has them until expiry. If you only want authenticated playback, leave public links disabled. A private-only VPN address can work for your players, but does not satisfy the bot's public-link requirement.

See [Navidrome sharing](https://www.navidrome.org/docs/usage/features/sharing/) for its configuration and proxy requirements.
