---
title: Troubleshooting
description: Check common startup, upload, import, library, and listening-link failures.
---

From `deploy`, begin with `docker compose ps` and `docker compose logs --tail=100 bot`. Also check the affected service's logs. When sharing logs, remove tokens, passwords, private URLs, and personal data.

## The bot does not start

Read the configuration errors: the loader reports missing environment values and unknown TOML keys. Confirm that `config.toml` is a file, not a directory accidentally created by a missing bind mount.

Check the [Postgres](https://www.postgresql.org/) health status. In the deployment sample, `POSTGRES_PASSWORD` configures a new database; changing that environment value later does not change an existing Postgres role's password. Update the role and connection settings together.

If [Navidrome](https://www.navidrome.org/) is unavailable, the bot may start while logging library-creation failures. Create the Navidrome administrator first, correct its credentials, then restart the bot so it can finish startup work.

## Telegram polling conflicts

Only one bot process should poll a token. Stop another development or production instance using the same token. When moving to the local API, perform cloud logout first and check local API credentials and logs. Do not run a manual `getUpdates` request while beatstash is polling.

## Uploading fails

Read the message beside the thumbs-down reaction. Check the file's format and integrity, your personal quota, and the server's free disk space. For larger files, confirm that the bot is using the local API and shares its file volume.

For filesystem errors, check write access to the bot's music directory and read access from Navidrome. Keep `shared`, `users`, and `.beatstash` on one filesystem. An `invalid cross-device link` error indicates an unsupported mount layout.

## A track is missing from Navidrome

A successful upload means the file is in the bot's library; Navidrome still needs to index it. Check the library's path in Navidrome and the corresponding container mount. Allow the scanner to finish, or trigger a scan through Navidrome's administration interface.

If a duplicate was skipped, the matching track may already be in an attached library. Look there before uploading it again.

## Existing music is missing from the bot

Check that the linked Navidrome account can see the library. The library must be separate from the bot's managed folders. A library covering all of the bot's music directory is ignored to prevent exposure of personal libraries.

Set `navidrome.attach_interval` to a positive duration; `0` disables attached libraries. Records refresh at startup and periodically, after Navidrome has indexed the files. The default refresh period is one hour, and access permissions are cached for up to 30 seconds.

## Inline results or listening links are missing

Verify BotFather inline mode, and that `/setinlinefeedback` is **Enabled**: with 1/100 or 1/10, most tracks picked in np or search stay at ⏳. Confirm a Navidrome account is linked. An imported or attached track may need a Telegram file prepared in the storage chat, or a public listening link as a fallback.

For links, check `navidrome.public_url`, sharing enabled in Navidrome, and proxy access to `/share`. Test a generated link from a private browser window outside your server network. Localhost, private IPs, and local-only names disable links in the bot.

If a storage chat is configured, check its numeric ID and the bot's posting permission.

## Zvuk connection or import fails

Reconnect with a fresh token and confirm the account has an active subscription. If there is no **Start the import** action because all collection tracks are already present, check the existing library rather than expecting duplicate downloads.

Look at the import summary for failed, unavailable, or over-quota tracks. Stars and playlists wait for Navidrome indexing. After a quota increase, a later sync retries tracks that did not fit.

If fresh credentials fail repeatedly, Zvuk's web API may have changed. Record the symptom and sanitized logs for an issue; the integration is unofficial.

## Saved credentials cannot be decrypted

Confirm that `SECRET_KEY` is the original key used with this database. A newly generated key cannot decrypt the old values.
