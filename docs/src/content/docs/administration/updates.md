---
title: Update your installation
description: Update one part of the stack at a time and verify the result.
---

Run deployment commands from `deploy`. The bot runs database migrations at startup. An older application may not understand a database changed by a newer one, so check the release notes for migration requirements before updating.

## Update beatstash

Read the [release notes](https://github.com/lubaskinc0de/beatstash/releases), including configuration and migration changes.

Download the new release's installation archive into a separate directory. Compare its `config.example.toml` and `compose.yml` with your own files, and apply any required changes. Keep your `.env`, database password, and `SECRET_KEY`; do not replace them with the empty templates.

Stop the bot, then edit `BEATSTASH_VERSION` in `.env` to the chosen version without the leading `v`. The latest stable version is `{{release_version}}`:

```sh
docker compose stop bot
```

After saving `.env`, pull that version and start it:

```sh
docker compose pull bot
docker compose up -d bot
docker compose logs --tail=100 bot
```

If the pull fails, fix the image name or connection before proceeding. Confirm that the running container uses the version you selected with `docker compose images bot`. Use exact versions for updates; `latest` follows the newest stable release, and prerelease images have their own tags such as `1.2.3-rc.1`.

Play a track, upload a small file, search through inline mode, and check a linked account. Check import status without starting a large new import just to test the update.

## Update other services

The sample pins Navidrome to `0.64.1`, matching the project's current test image. Change its tag deliberately after reviewing Navidrome's release and compatibility notes. If you already run Navidrome elsewhere, update it through that installation's own procedure.

The local Telegram API image uses `latest`. Record its current digest before pulling an update. Update services separately so you can identify which change caused a failure:

```sh
docker compose pull telegram-bot-api
docker compose up -d telegram-bot-api
docker compose logs --tail=100 telegram-bot-api
```

Restart the bot if its connection does not recover. After choosing a tested image digest, you can replace the mutable tag with that digest in your local Compose file.

The Postgres sample uses major version 17. A major-version database upgrade requires a separate migration procedure; changing the image to a different major is not a database upgrade plan.

## Roll back

If only the application changed and its database is compatible with the previous version, set `BEATSTASH_VERSION` back to that version, then run `docker compose pull bot` and `docker compose up -d bot`. If a release includes incompatible database migrations, switching the image tag alone cannot roll back the database.

Keep the bot stopped while resolving a failed update. Do not remove volumes as a troubleshooting shortcut: they contain the database and Telegram API state.
