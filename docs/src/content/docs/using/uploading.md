---
title: Upload music
description: Add audio files from Telegram, understand reactions, and handle duplicates or missing tags.
---

Send an audio file or an audio document to the bot in a private chat. You can also forward a file from another conversation. Supported formats are **MP3, FLAC, M4A, OGG, Opus, and WAV**. WAV uploads are converted to FLAC without losing audio quality.

| Reaction | Meaning |
|---|---|
| 👀 | The bot accepted the file and is processing it |
| 👍 | Processing finished successfully |
| 👎 | Processing failed; read the accompanying message |

New uploads go into your personal library. They appear in [Navidrome](https://www.navidrome.org/) after its scanner indexes them. Uploading does not publish a track in the shared library or send it to other chats.

## Track details and artwork

The bot fills in available artist, title, album, album artist, year, and track number, then writes those tags into the audio file. That gives Navidrome and your player the same track details the bot uses.

For each field, it uses the first source that provides a value:

1. Details from the import provider.
2. Tags already in the audio file.
3. Artist and title supplied by Telegram.
4. An `Artist - Title` filename.

For example, an MP3 with no tags can still get its artist and title from Telegram or its filename. Existing file tags take priority over Telegram's labels. The bot cleans up extra whitespace and uses the track artist as album artist when that field is missing.

Zvuk imports also supply album details, year, track number, and cover artwork when available. The bot embeds that artwork in the downloaded file. New music is placed into artist and album folders; singles get their own folder.

The bot does this automatically when you add music. You cannot edit tags manually in the bot yet. Files in your existing Navidrome libraries keep their original tags.

## Duplicates

The bot checks metadata and duration to detect matching tracks. In its managed personal library, a better-quality copy can replace a lower-quality one: lossless beats lossy, and higher bitrate wins between lossy copies.

If a matching track exists in an attached Navidrome library you can access, the bot skips the new copy regardless of its quality. It leaves that existing file untouched.

## Missing artist or title

Files without enough metadata go into **Inbox**. You can keep and listen to them, but sharing requires an artist and title. Interactive tag editing through the bot is not available yet.

Tag files before uploading when possible. If the server owner edits a managed file on disk, the bot picks up the change through its periodic reconciliation. Keep file-management work with the server owner.

## Upload failures

The common causes are a full personal quota, an unsupported or damaged file, and Telegram's file-size limit. Read the bot's error message first. Retry a Telegram download failure; ask the administrator about quotas or persistent processing errors.

See [libraries and quotas](./libraries.md) and [server troubleshooting](../administration/troubleshooting.md).
