---
title: Import and sync Zvuk
description: Bring your saved Zvuk collection into Navidrome and understand how later changes are synced.
---

## Connect your account

You need an active Zvuk subscription and enough room in your personal library.

1. Sign in at [zvuk.com](https://zvuk.com) in a browser.
2. Open [the profile endpoint](https://zvuk.com/api/tiny/profile) in that signed-in browser and copy the `token` field.
3. In the bot, open **Import from a service > Zvuk > Connect**.
4. Send the token when asked. The bot deletes that input message and stores the token encrypted.

Treat the token as an account credential. If the profile response has no usable token or Zvuk rejects it, sign in again and retry.

## Start the first import

Choose **Import** to see the plan: saved tracks, tracks missing from your library, an estimated download size, and available quota. Choose **Start the import** to begin.

The bot downloads liked tracks, saved album tracks, and playlist tracks. Likes become stars in [Navidrome](https://www.navidrome.org/). Playlists keep their names and track order. Available FLAC is preferred; otherwise the provider uses its high-quality MP3 stream.

Downloads include pauses and are limited per account, so a large collection takes time. View **Import progress** to see results. Navidrome must index downloaded files before stars and playlists can be applied; a playlist may be incomplete while that happens.

A track already in your personal library or an attached library you can access may be skipped as a duplicate. The existing Navidrome track is then used for stars and playlists.

## What sync does

Automatic sync begins after the first import. Connecting a token alone does not start it. Its default interval is six hours. It runs from Zvuk to Navidrome; changes you make in Navidrome are not sent back.

| Change in Zvuk | Result in Navidrome |
|---|---|
| Like a new track or save more collection music | Missing tracks are downloaded; likes become stars |
| Remove a like | The star is removed; downloaded audio stays |
| Add or remove playlist tracks | The mirrored playlist's track list is updated; downloaded audio stays |
| Rename a playlist | The existing Navidrome playlist keeps its current name |
| Delete a playlist | Its Navidrome copy is not deleted |

This is not a full mirror of every provider action. The initial likes keep their order, and application of later stars may wait for earlier tracks to finish downloading and indexing.

## Missing tracks and full quotas

Tracks unavailable from Zvuk can be skipped. Other downloads can fail or run out of personal quota. Review the import summary and confirm that important albums and playlists play.

If the administrator increases your quota, a later sync retries tracks that did not fit. A copied playlist cannot include a track until Navidrome has indexed a usable file.

## Disconnect or reconnect

Choose **Disconnect** on the Zvuk screen to delete the saved token and stop new imports from that connection. Downloaded files remain. Saved collection state is retained so the connection can continue when you reconnect.

If the token stops working, the bot notifies you. Use **Connect again** and provide a fresh token.

The integration uses unofficial Zvuk web endpoints. Provider changes can break connection, downloads, or sync. See [troubleshooting](../administration/troubleshooting.md) if reconnecting does not resolve the issue.
