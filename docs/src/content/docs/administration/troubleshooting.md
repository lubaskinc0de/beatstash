---
title: Troubleshooting
description: Find the cause of common startup, upload, import, library, and listening-link failures.
---

Start in `deploy` with `docker compose ps` and `docker compose logs --tail=100 bot`, then check the logs of the service that misbehaves.

:::caution
Before sharing logs, remove tokens, passwords, private addresses, and personal data.
:::

## The bot doesn't start

The log names missing environment values and unknown TOML keys. Check that `config.toml` is a file: a missing bind mount makes Docker create a directory in its place.

Check that [Postgres](https://www.postgresql.org/) is healthy. `POSTGRES_PASSWORD` sets the password only when the database is first created; changing it later doesn't change the existing role's password. Change the role and the setting together.

If [Navidrome](https://www.navidrome.org/) is down, the bot may start but log failures to create libraries. Create the Navidrome administrator, fix its credentials, and restart the bot so it finishes its startup work.

If the log shows `getMe` timeouts, the server can't reach Telegram; see [when Telegram is blocked](./blocked-telegram.md).

## Two bots fight over updates

Only one process may poll a token. Stop any other development or production copy that uses it. When moving to the local Bot API, log out of the cloud API first and check the local API's credentials and logs. Don't call `getUpdates` by hand while the bot is running.

## Uploading fails

Read the message next to 👎. Check the file's format and integrity, your quota, and the server's free disk space. For large files, make sure the bot uses the local Bot API and shares its file volume.

For file system errors, check that the bot can write to its music folder and Navidrome can read it. An `invalid cross-device link` error means the [managed folders](../reference/glossary.md#managed-folders) are on different filesystems.

## A track is missing from Navidrome

A successful upload means the file is in the bot's library; Navidrome still has to scan it. Check the library path in Navidrome and the matching container mount. Wait for the scan, or start one from Navidrome's admin screens.

If the upload was skipped as a duplicate, the track may already be in an existing library. Look there first.

## Existing music is missing from the bot

Check that the linked Navidrome account can see the library, and that the library doesn't overlap the bot's managed folders. A library covering the whole music folder is ignored, because it would show everyone's personal libraries.

`navidrome.attach_interval` must be above `0`; `0` turns existing libraries off. The bot refreshes them at startup and then hourly, after Navidrome has scanned the files. Access changes take up to 30 seconds to reach the bot.

## Inline results or listening links are missing

Check that inline mode is on in BotFather and `/setinlinefeedback` is **Enabled**: with 1/10 or 1/100, most tracks picked in `np` or a search stay at ⏳. Check that the person has a linked Navidrome account. An imported track, or one from an existing library, may need the [storage chat](./storage-chat.md) or a listening link.

For listening links, check `navidrome.public_url`, that sharing is on in Navidrome, and that the proxy passes `/share`. Open a generated link in a private window from outside your network. `localhost`, private IP addresses, and local-only names turn links off.

If you use a storage chat, check its numeric ID and that the bot may post there.

## Zvuk connection or import fails

Reconnect with a fresh token and check that the subscription is active. If there is no **Start the import** button, every track in the collection is already in your library.

The import summary lists failed, unavailable, and over-quota tracks. Stars and playlists wait until Navidrome scans the files. After a quota increase, the next sync retries tracks that didn't fit.

If fresh tokens keep failing, Zvuk may have changed its unofficial API. Open an issue with the symptom and cleaned-up logs.

## Saved credentials can't be decrypted

`SECRET_KEY` must be the key this database was used with. A newly generated key can't decrypt old values.
