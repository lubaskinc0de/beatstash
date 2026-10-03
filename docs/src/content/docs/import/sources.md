---
title: Supported sources
description: What you can bring into your library and which streaming integrations are available.
---

You can upload your own audio files, import a supported streaming collection, or use music already on your [Navidrome](https://www.navidrome.org/) server. Support for one service does not imply support for another.

| Source | Audio | Likes | Saved albums | Playlists | Automatic sync | Requirements |
|---|---|---|---|---|---|---|
| Telegram files | Upload or forward | No import | Upload individual files | No import | No | Supported format; file fits Telegram limits and your quota |
| Zvuk | Collection download | Navidrome stars | Album tracks downloaded | Names and track order imported | Yes, after the first import | Token and active subscription |
| Existing Navidrome | Existing tracks registered; files stay in place | Existing Navidrome state remains | Existing collection remains | Existing Navidrome state remains | Refresh of library records | Server administrator connects beatstash; account has access |
| Yandex Music | Not available yet | No | No | No | No | Planned integration |
| YouTube Music | Not available yet | No | No | No | No | Planned integration |
| Spotify | Not available yet | No | No | No | No | Planned integration |
| Apple Music | No built-in import | No | No | No | No | Integration not implemented |
| Other streaming services | No built-in import | No | No | No | No | Check this table when integrations are added |

## Existing files

If your files already live in Navidrome, the administrator can [connect that server](../installation/existing-navidrome.md). The bot adds the libraries and tracks to its own database without moving the originals.

If you have files on a computer or phone, [send them through Telegram](../using/uploading.md). This does not carry over a streaming service's likes, listening history, or playlists.

## Streaming collections

[Zvuk import](./zvuk.md) currently handles the saved collection. Sending the bot a Zvuk track, album, or playlist URL does not import it. Other streaming services do not have direct import support yet.

Yandex Music, YouTube Music, and Spotify integrations are planned. No release dates or specific import capabilities have been announced for them. Their entries will be updated when support is implemented and checked.

Check the collection before cancelling a subscription: confirm that the expected tracks downloaded and play through Navidrome. Unavailable tracks, download failures, and quotas can leave an import incomplete.

The downloaded audio remains after you disconnect the source. Adding new music later still requires files or a supported connected source.
