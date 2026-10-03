---
title: Share music
description: Send audio in Telegram, publish to the shared library, or create a public listening link.
---

## Send music to a Telegram chat

In any chat, type your bot's username followed by a query, then select a result. Replace `your_music_bot` below with your own bot's username:

```text
@your_music_bot massive attack
@your_music_bot np
@your_music_bot recent
@your_music_bot shared
@your_music_bot top
```

| Query | Results |
|---|---|
| Any track, artist, or album name | Search your personal and accessible attached libraries |
| `np` | What your linked [Navidrome](https://www.navidrome.org/) account is playing now |
| `recent` or `last` | Your ten most recent tracks |
| `shared` | Recent tracks in the shared library |
| `top` | Participants who share the most |

An empty query shows hints. A result may send an audio file or a listening link, depending on whether a Telegram file is available and how the server is configured. Select the result and allow the bot time to prepare it.

Inline sending does not add the track to the server's shared library. Anyone who receives an audio file can keep or forward it.

## Publish to the shared library

Open **Music > Mine**, find a track or album, and choose **Share** or **Share the album**. Other participants can find it in **Music > Shared** and choose:

- **Take** to add the track to their personal library.
- **Send the file** to receive it in Telegram.

Choose **Remove from Shared** to withdraw your publication. Another participant's own publication may keep the track in the shared library. Copies people already took remain in their personal libraries.

Tracks without an artist or title cannot be published. The shared quota may also prevent publication.

## Public listening links

An album card offers **Listening link**. Inline results may also use links for tracks that cannot be sent as audio.

These are Navidrome links that work without an account. Anyone with the link can listen until it expires; downloading may also be allowed. The server defaults to a 30-day lifetime and allows downloads, but the administrator can change both.

A public link does not publish music in the shared library. Treat sending it as giving access to the people who receive or forward it. The administrator must configure a public Navidrome address and enable sharing for links to work.
