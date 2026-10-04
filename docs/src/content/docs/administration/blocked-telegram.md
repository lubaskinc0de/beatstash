---
title: When Telegram is blocked
description: Route the local Bot API through a proxy when the server can't reach Telegram.
---

The local Bot API connects straight to Telegram's data centers. If Telegram is blocked in the server's country or by the hosting provider, it can't sign in, and the bot fails to start with `getMe` timeouts in its log. Check from the server:

```sh
curl -m 10 -sS -o /dev/null https://api.telegram.org && echo reachable
```

The installer runs the same check. Only if Telegram is unreachable does it ask for a proxy. You can skip the question; the bot will start once the server reaches Telegram.

## Pick the proxy address

The proxy is `http://host:port` or `socks5://host:port`. Usually it is a VPN client on the same server, such as [xray](https://xtls.github.io/en/), [v2ray](https://www.v2fly.org/en_US/), or [sing-box](https://sing-box.sagernet.org/). Take the protocol and port of its HTTP or SOCKS inbound from the `inbounds` section of its configuration.

Containers reach the server as `host.docker.internal`, so the inbound must listen on the [Docker](https://docs.docker.com/) gateway `172.17.0.1`, not only on `127.0.0.1`. For an xray HTTP inbound on port 10809:

```json
{ "protocol": "http", "listen": "172.17.0.1", "port": 10809 }
```

Check it with `curl -x http://172.17.0.1:10809 https://api.telegram.org`, then give the installer `http://host.docker.internal:10809`.

## Turn it on by hand

The Bot API's own `--proxy` option covers webhooks only. So the deployment routes the whole Bot API container through the proxy: `compose.telegram-proxy.yml` adds a small `telegram-proxy` container ([tun2socks](https://github.com/xjasonlyu/tun2socks)) whose network the Bot API shares. Other services still connect directly.

Add to `deploy/.env`:

```dotenv title="deploy/.env"
COMPOSE_FILE="compose.yml:compose.telegram-proxy.yml"
TELEGRAM_PROXY="http://host.docker.internal:10809"
```

Then run `docker compose up -d`. The server needs `/dev/net/tun`, which most virtual servers have. The installer's logout from the cloud API goes through the same proxy.
