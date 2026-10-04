---
title: HTTPS and public access
description: Put Navidrome behind a reverse proxy with HTTPS, so players and listening links work from anywhere.
---

The deployment sample exposes [Navidrome](https://www.navidrome.org/) only on `127.0.0.1:4533`, so out of the box you can listen only through an SSH tunnel. To open it from your phone and to send [listening links](../reference/glossary.md#listening-link), Navidrome needs an HTTPS address behind a reverse proxy. The bot itself uses long polling and needs no inbound port.

## Before you start

You need:

- a domain or subdomain, such as `music.example.com`, with an A record pointing to the server's public IPv4 address (add an AAAA record only if the server really accepts IPv6);
- ports 80 and 443 open at your hosting provider and in the server's firewall. Caddy and Certbot use port 80 to obtain the certificate, even if all traffic then goes over HTTPS.

Only Navidrome goes behind the proxy. Keep [Postgres](https://www.postgresql.org/) and the local Telegram Bot API off the public network.

## With the installer

When you install on a new server and enter an `https://` public address, the installer offers to set up HTTPS. It explains each step before running it, and you can skip any of them. What it does depends on what already runs on the server.

If [Caddy](https://caddyserver.com/docs/) already serves other sites here, as a container on the host network or a systemd service, the installer appends a site for your domain to its Caddyfile between `# beatstash:begin` and `# beatstash:end`, then reloads Caddy. Other sites keep working through the reload. Before writing, it checks that:

- no other site in that Caddyfile serves the domain;
- the running Caddy config matches the file, so the reload won't undo changes made through Caddy's admin API;
- Caddy accepts the edited file.

It shows the change and asks twice: before writing and before reloading. The old file is kept as `Caddyfile.backup` in `deploy`, and a failed reload restores it. For a systemd service it uses `sudo`.

If nothing uses ports 80 and 443, the installer can start Caddy as part of the beatstash stack, with the site in `deploy/Caddyfile`. You can give an email for certificate notices.

In any other case, such as [nginx](https://nginx.org/en/docs/), [Traefik](https://doc.traefik.io/traefik/), Caddy on a [Docker](https://docs.docker.com/) network, or another program on those ports, the installer prints the address to proxy to and sends you to the [manual setup](#manual-setup).

At the end it checks that `https://your-domain/ping` answers. If it doesn't, it shows how the domain resolves and Caddy's latest log lines. A new DNS record can take a while to spread, so you can check again or skip; Caddy keeps retrying the certificate in the background.

Uninstalling removes the beatstash site from that Caddyfile, with the same checks.

## Manual setup

Any reverse proxy that terminates TLS will do. It accepts `https://music.example.com` and forwards every request to Navidrome.

### 1. Choose the upstream address

Where the proxy finds Navidrome depends on where the proxy runs:

| Proxy runs | Upstream address |
| --- | --- |
| On the host, or in a container with `network_mode: host` | `127.0.0.1:4533` |
| In a container on its own Docker network | Attach it to the `beatstash_default` network and use `navidrome:4533` |
| On another machine | Publish Navidrome's port on a private interface or VPN address and use that |

Inside an ordinary container, `127.0.0.1` is the container itself, not the host.

:::danger
Never expose port `4533` to the internet without TLS.
:::

### 2. Configure the proxy

The proxy must:

- obtain and renew a certificate for the domain, for example from Let's Encrypt;
- forward every path, including `/share/...` and `/rest/...`, without an extra login prompt: listening links have to open for people without an account, and player apps sign in on their own;
- pass the original `Host` and the client address (`X-Forwarded-For`, `X-Forwarded-Proto`);
- allow long responses and large bodies for streaming and downloads.

[Caddy](https://caddyserver.com/docs/) handles certificates and headers itself:

```text title="/etc/caddy/Caddyfile"
music.example.com {
    reverse_proxy 127.0.0.1:4533
}
```

Check and apply it with `caddy validate --config /etc/caddy/Caddyfile` and `systemctl reload caddy`, or `docker exec <container> caddy reload --config /etc/caddy/Caddyfile` for a container.

[nginx](https://nginx.org/en/docs/) needs a certificate from a tool such as [Certbot](https://certbot.eff.org/):

```nginx title="/etc/nginx/sites-available/music.example.com"
server {
    listen 443 ssl;
    server_name music.example.com;
    ssl_certificate     /etc/letsencrypt/live/music.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/music.example.com/privkey.pem;
    client_max_body_size 0;

    location / {
        proxy_pass http://127.0.0.1:4533;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_buffering off;
        proxy_read_timeout 1h;
    }
}
```

Add a `listen 80` server that redirects to HTTPS, test with `nginx -t`, and reload. Traefik and other proxies need the same rules in their own labels or configuration.

To run Caddy inside the beatstash stack instead, add `proxy` to `COMPOSE_PROFILES` in `deploy/.env` (for example `COMPOSE_PROFILES="navidrome,proxy"`), write `deploy/Caddyfile` with `reverse_proxy navidrome:4533`, and run `docker compose up -d`. Ports 80 and 443 must be free.

### 3. Check access

```sh
curl -fsS -o /dev/null -w '%{http_code}\n' https://music.example.com/ping
```

It should print `200`. If it doesn't, check that the domain resolves to the server (`dig +short music.example.com`), that ports 80 and 443 are open from outside, and the proxy's logs.

## Set the public address

The installer does this for you. Otherwise, edit the Navidrome section:

```toml title="deploy/config.toml" ins={3}
[navidrome]
url = "http://navidrome:4533"
public_url = "https://music.example.com"
user = "admin"
```

Keep your own internal `url` and login, then [apply the change](../administration/configuration.md#change-a-setting):

```sh
docker compose up -d --force-recreate bot
```

The new-server sample already sets `ND_ENABLESHARING=true`. For an existing Navidrome, turn on sharing in its own configuration.

The bot turns listening links off when `public_url` is `localhost`, a private IP address, or a name that only resolves locally. A VPN-only address works for your own players but not for listening links.

## Verify access

1. Open `https://music.example.com` from outside the server's network and sign in.
2. Add that address to your player app and play a track.
3. Request an album's **Listening link** in the bot.
4. Open the link in a private browser window, without a Navidrome session.
5. Check that downloading is allowed or blocked as `listen_link_downloadable` says.

Anyone with a listening link can play the music until it expires. If you want only signed-in playback, leave listening links turned off. Navidrome describes its side in [sharing](https://www.navidrome.org/docs/usage/features/sharing/).
