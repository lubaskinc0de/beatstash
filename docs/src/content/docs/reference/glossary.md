---
title: Glossary
description: The terms these docs use, and the names the bot and Navidrome give the same things.
---

### Administrator

A Telegram account listed in `admins`. Administrators invite people, set quotas, and see the **Admin** screen. The bot calls them the admin or the server's owner. A *Navidrome administrator* is a different role: a Navidrome account that sees every library and can create accounts.

### Participant

Anyone with an account in the bot: the administrator and everyone they invited. Each participant has a Navidrome account and a personal library.

### Personal library

A participant's own folder of music, under `users` in the managed folders. Uploads and imports go here. Only its owner and Navidrome administrators see it.

### Shared library

One library for the whole server, under `shared`. Participants put tracks here on purpose, and others can take copies into their personal libraries. The bot calls it the Shared Library.

### Existing library

A Navidrome library that holds music outside the bot's managed folders, usually a collection you had before installing the bot. The bot reads its tracks but never changes the files. Configuration keys call this attaching, as in `navidrome.attach_interval`.

### Managed folders

The folders the bot writes to under `library.music_dir`: `shared`, `users`, and the bot's working folder `.beatstash`. They must be on one filesystem.

### Inbox

Where tracks without an artist or title end up. They can be played but not shared.

### Quota

A limit on the total size of a personal library, or of the shared library. Existing libraries don't count. See [libraries and quotas](../using/libraries.md#what-a-quota-measures).

### Listening link

A Navidrome share link. Anyone who has it can play the music without an account until it expires. It needs a public HTTPS address; see [HTTPS](../installation/https.md).

### Inline mode

Calling the bot from any Telegram chat by typing `@your_music_bot` and a query. See [sharing](../using/sharing.mdx#send-music-to-any-chat).

### Storage chat

A private Telegram channel where the bot uploads imported and existing tracks, so it can send them as audio. See [storage chat](../administration/storage-chat.md).

### Local Bot API

A Telegram Bot API server running next to the bot. It lets the bot handle files up to 2000 MiB instead of the cloud API's 20 MB and 50 MiB limits. See [Set up Telegram](../installation/telegram.mdx#run-the-local-bot-api).

### Import and sync

An import downloads a streaming collection into your personal library. Sync runs after the first import and brings in what you saved on the service since. See [supported sources](../import/sources.md).

### Installer

`install.sh` from a release. It installs, finishes an interrupted setup, upgrades (`bash install.sh upgrade`), and uninstalls (`bash install.sh uninstall`).
