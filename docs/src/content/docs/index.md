---
title: Your Navidrome library, through Telegram
description: Upload music, import your collection, invite friends, and share tracks in Telegram with beatstash.
---

beatstash helps you build and manage a Navidrome music library through Telegram. Bring over a collection from a supported streaming service, upload new music, and share tracks or what you're listening to in your chats. You can connect an existing Navidrome server or set up a new one.

## Already have Navidrome?

Navidrome scans audio files on your server and makes them available in its web player and compatible apps. It has accounts, library permissions, playlists, and public sharing links. Its standard workflow does not import your streaming collection or turn a Telegram attachment into a library track. You still need to get files onto the server and manage access to them.

### Add music from Telegram

A friend sends you a track in Telegram, and you want it in Navidrome too. Forward it to beatstash, and the bot adds it to your personal library with its artist, title, and album details. If you already have the track, it avoids adding another copy. Once Navidrome scans it, you can listen in your usual player. You can do the same with an album's audio files on your phone, without opening an SSH session or transferring them to the server yourself.

### Share music in your chats

Share your taste in the chats you already use: show friends what you're listening to, reply to a recommendation with a track of your own, or send a favorite album for them to try. Type `@your_music_bot np` to share what's playing, or search your collection by artist or title right in the conversation. Friends receive audio or a listening link, so they can hear the recommendation instead of looking up a song name themselves. See [sharing in Telegram](./using/sharing.md) for examples.

### Run a music server with friends

Send a friend an invite, and the bot helps them set up an account with a library of their own. They can add music from their phone and listen through Navidrome, without setting up another server or asking you to upload every file.

Each person chooses what to share with the group. For example, a friend adds an album and shares a track they liked. You can listen to it, take it into your own library, or send it to a Telegram chat. Personal collections stay separate, and you can [limit how much space each person uses](./using/libraries.md#change-quotas-as-an-administrator).

[Connect the bot to the Navidrome server you already use](./installation/existing-navidrome.md). Your music stays where it is, and you can find and share it through Telegram. Music you send to the bot is added to your personal collection on the same server.

## Starting with a streaming collection?

Moving to your own library is easier when you can bring your saved music with you. With a supported provider, connect your account in the bot, review the collection, and start the import. beatstash downloads the music to your server and carries over supported likes and playlists, so you do not have to add every track by hand.

After the move, keep adding files through Telegram, search your collection from a chat, or send friends the song you're playing. For a connected provider, periodic sync adds new collection music after the first import. Playback happens in Navidrome or your preferred compatible app.

### Provider support

| Provider | Status |
|---|---|
| Zvuk | Available: collection import and periodic sync |
| Yandex Music | Planned; not available yet |
| YouTube Music | Planned; not available yet |
| Spotify | Planned; not available yet |

See [supported sources](./import/sources.md) for what each import carries over and its requirements. Planned integrations have no announced release date. You can also upload audio files you already own, regardless of which service you used before.

Once you have the files, listen through Navidrome's web player or a compatible app. Disconnecting a streaming source leaves the downloaded music in your library. You pay for your server and storage; the bot and Navidrome have no subscription fee. Connecting Zvuk requires an active subscription.

[Install Navidrome and beatstash together](./installation/new-server.md) to start with a new server.

## Invited to someone's server?

Start with [joining and setting up your account](./using/getting-started.md). You do not need to install anything on a server.

Navidrome handles playback. beatstash handles getting music into the library and sharing it. See [how it compares with Navidrome and beets](./introduction/comparison.md) if you're choosing tools for your collection.
