---
title: Supported sources
description: What you can bring into your library, and which streaming services the bot imports from.
---

Moving to your own server is easier when your saved music comes with you. There are three ways to fill a library: import a streaming collection, upload files you have, or connect a Navidrome that already holds your music.

| Source | Audio | Likes | Playlists | Sync after the first import | You need |
|---|---|---|---|---|---|
| [Zvuk](./zvuk.mdx) | Liked tracks, saved albums, and playlist tracks downloaded | Become Navidrome stars | Names and track order copied | Yes, every 6 hours | A Zvuk token and an active subscription |
| [Telegram files](../using/uploading.mdx) | Each file you send or forward | Not carried over | Not carried over | No | A supported format within Telegram's limits and your quota |
| [Existing Navidrome](../installation/existing-navidrome.mdx) | Tracks stay where they are | Stay in Navidrome | Stay in Navidrome | Records refresh hourly | The server's administrator connects the bot |

Yandex Music, YouTube Music, and Spotify are planned and can't be imported yet. Other services, such as Apple Music, have no import. Their music can still go through Telegram as files, without likes or playlists.

## Before you cancel a subscription

Check the imported collection first: make sure the tracks you care about downloaded and play in Navidrome. Tracks the service won't serve, failed downloads, and a full quota can all leave gaps.

Music you imported stays in your library after you disconnect the service. After that, new music comes only from files or another connected service.

## Links to single tracks

Sending the bot a Zvuk link to a track, album, or playlist doesn't import it. The Zvuk import works on your saved collection as a whole.
