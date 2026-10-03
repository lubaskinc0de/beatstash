---
title: HTTPS and public access
description: Put Navidrome behind a reverse proxy with HTTPS and enable public listening links.
---

The deployment sample exposes Navidrome only on `127.0.0.1:4533`. To open it from your phone or another computer, and to send public listening links, it needs an HTTPS address served by a reverse proxy. beatstash itself uses Telegram long polling and needs no inbound webhook.

## Before you start

You need:

- a domain or subdomain, such as `music.example.com`, with an **A record** pointing to your server's public IPv4 address. Add an AAAA record only if the server actually accepts IPv6 connections;
- inbound ports **80 and 443** open at your hosting provider and in the server's firewall. Port 80 is needed for certificate checks even if all traffic uses HTTPS.

Keep Postgres and the local Telegram Bot API off the public network: only Navidrome goes behind the proxy.

## With the guided setup

When you install on a new server and enter an `https://` public listening URL, the installer offers to set up HTTPS. Every step is optional and explained before it runs.

- **Caddy already serves other sites on this server** (a container on the host network, or a systemd service): the installer adds a site for your domain at the end of its Caddyfile, between `# beatstash:begin` and `# beatstash:end` markers, and reloads Caddy. Other sites stay as they are and keep working during the reload. Before writing, it checks that:
  - the domain is not already served elsewhere in that Caddyfile;
  - the running Caddy config matches the file, so a reload cannot undo changes made through Caddy's admin API;
  - Caddy accepts the changed file.

  It then shows the change and asks before writing it and again before reloading. The previous file is saved as `Caddyfile.backup` in your installation's `deploy` directory, and a failed reload puts it back. For a systemd service, it uses `sudo`.
- **No proxy uses ports 80 and 443**: the installer can start Caddy as part of the beatstash stack, with its site in `deploy/Caddyfile`. You can give an email for certificate notices.
- **Anything else** (nginx, Traefik, Caddy on a Docker network, ports taken by another program): the installer prints the address to proxy to and links here. Follow the manual setup below.

Finally, it checks that `https://your-domain/ping` answers and, if not, shows how the domain resolves and Caddy's latest log lines. A new DNS record can take a while to propagate; you can check again or skip. Caddy keeps retrying the certificate in the background.

Uninstalling removes the beatstash site from that Caddyfile again, with the same checks.

## Manual setup

Any reverse proxy that terminates TLS works. Its job is to accept `https://music.example.com` and forward every request to Navidrome.

### 1. Choose the upstream address

Where the proxy reaches Navidrome depends on where the proxy runs:

| Proxy runs | Upstream address |
| --- | --- |
| On the host, or in a container with `network_mode: host` | `127.0.0.1:4533` |
| In a container on its own Docker network | Attach it to the `beatstash_default` network and use `navidrome:4533` |
| On another machine | Publish Navidrome's port on a private interface or VPN address and use that; never expose `4533` to the internet without TLS |

`127.0.0.1` inside an ordinary container is the container itself, not the host.

### 2. Configure the proxy

The proxy must:

- obtain and renew a certificate for the domain, for example from Let's Encrypt;
- forward all paths, including `/share/...` and `/rest/...`, without an extra login prompt: public links must open for people who have no account, and player apps authenticate on their own;
- pass the original `Host` and the client address (`X-Forwarded-For`, `X-Forwarded-Proto`);
- allow long responses and large bodies for streaming and downloads.

**Caddy** handles certificates and headers by itself:

```text
music.example.com {
    reverse_proxy 127.0.0.1:4533
}
```

Check and apply it with `caddy validate --config /etc/caddy/Caddyfile` and `systemctl reload caddy`, or `docker exec <container> caddy reload --config /etc/caddy/Caddyfile` for a container.

**nginx** needs a certificate from a tool such as Certbot:

```nginx
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

Add a `listen 80` server that redirects to HTTPS, test with `nginx -t`, and reload. For Traefik and other proxies, apply the same rules with their own labels or configuration.

To run Caddy as part of the beatstash stack instead, add `proxy` to `COMPOSE_PROFILES` in `deploy/.env` (for example `COMPOSE_PROFILES="navidrome,proxy"`), write `deploy/Caddyfile` with `reverse_proxy navidrome:4533`, and run `docker compose up -d`. Ports 80 and 443 must be free.

### 3. Check access

```sh
curl -fsS -o /dev/null -w '%{http_code}\n' https://music.example.com/ping
```

It should print `200`. If it fails, check that the domain resolves to the server (`dig +short music.example.com`), that ports 80 and 443 are open from outside, and the proxy's logs.

## Set the public address

The guided setup does this for you. Otherwise, edit the existing Navidrome section in `deploy/config.toml`:

```toml
[navidrome]
url = "http://navidrome:4533"
public_url = "https://music.example.com"
user = "admin"
```

Keep your actual internal URL and login, then [apply the change](./configuration.md#change-a-setting):

```sh
docker compose up -d --force-recreate bot
```

The new-server sample already sets `ND_ENABLESHARING=true`. For an existing Navidrome, enable sharing in its own configuration.

## Verify access

1. Open `https://music.example.com` from outside your server's network and sign in.
2. Add that address to your listening client and play a track.
3. Request an album's **Listening link** in the bot.
4. Open the link in a private browser window without a Navidrome session.
5. Check whether downloading is available according to `listen_link_downloadable`.

Links work for anyone who has them until expiry. If you only want authenticated playback, leave public links disabled. A private-only VPN address can work for your players, but does not satisfy the bot's public-link requirement.

See [Navidrome sharing](https://www.navidrome.org/docs/usage/features/sharing/) for its configuration and proxy requirements.
