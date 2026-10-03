---
title: Uninstall
description: Remove a beatstash installation, keeping as much of its data as you choose.
---

Run the `uninstall` command of the installer on the server. Any release's installer works:

```sh
curl -fL https://github.com/lubaskinc0de/beatstash/releases/download/{{release_tag}}/install.sh -o install.sh
curl -fL https://github.com/lubaskinc0de/beatstash/releases/download/{{release_tag}}/install.sh.sha256 -o install.sh.sha256
sha256sum -c install.sh.sha256
bash install.sh uninstall
```

It finds the running installation, asks you to confirm it, and then asks about each part separately. Enter takes the default shown in brackets:

| Part | Default | What goes away |
| --- | --- | --- |
| Return the bot to Telegram's cloud API | no | Logs the bot out of the local Bot API, so it can run elsewhere on the cloud API, which can take up to 10 minutes to accept it |
| Containers and network | yes | The running services; the downloaded images stay |
| The beatstash site in a shared [Caddy](https://caddyserver.com/docs/) | yes | The block between the `beatstash` markers, with the same checks as when it was added; see [HTTPS](./https.md#with-the-guided-setup) |
| Volumes | no | The bot's database and the local Bot API's data: accounts, libraries, links, imports |
| [Navidrome](https://www.navidrome.org/)'s data | no | Navidrome's own database: its users, playlists and play counts |
| Music | no | Everything uploaded and imported through the bot; the question shows its size |
| Installation directory | no | `.env` and `config.toml`, offered only when everything above is gone |

Data is offered for deletion only after the containers are removed. Whatever you keep stays usable: run the installer in the same directory to start again with it.

A Navidrome you connected as an existing server is never touched, and neither is the music it had before. Remove the beatstash music mount and libraries from it yourself if you no longer need them.

## By hand

From `deploy`, `docker compose down` stops and removes the containers, and `docker compose down -v` also deletes the volumes. Delete the installation directory afterwards; files written by the containers belong to root, so use `sudo rm -rf` for it. Remove the site you added to your reverse proxy and reload it.
