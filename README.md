# beatstash

[![CI](https://img.shields.io/github/actions/workflow/status/lubaskinc0de/beatstash/ci.yml?branch=master&label=CI)](https://github.com/lubaskinc0de/beatstash/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/lubaskinc0de/beatstash)](https://github.com/lubaskinc0de/beatstash/releases)
[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/lubaskinc0de/beatstash/badges/coverage.json)](https://github.com/lubaskinc0de/beatstash/actions/workflows/ci.yml?query=branch%3Amaster)
[![License: MIT](https://img.shields.io/github/license/lubaskinc0de/beatstash)](LICENSE)

**[Documentation](https://lubaskinc0de.github.io/beatstash/)** · [Install](https://lubaskinc0de.github.io/beatstash/installation/requirements/) · [Join a server](https://lubaskinc0de.github.io/beatstash/using/getting-started/) · [Configuration](https://lubaskinc0de.github.io/beatstash/reference/configuration/) · [Troubleshooting](https://lubaskinc0de.github.io/beatstash/administration/troubleshooting/)


A self-hosted Telegram bot that fills your [Navidrome](https://www.navidrome.org/) music library, lets friends in, and shares music in your chats.

https://github.com/user-attachments/assets/10977953-8934-4261-9c3b-bfe9485822a7

Navidrome is a good way to listen to your own music, but filling it is up to you. A friend sends a track in Telegram, and you copy it to the server over SSH. A friend wants in, and you create their account, pick their libraries, and upload their files. You leave a streaming service, and your likes and playlists stay behind. 

**Beatstash** moves that work into a Telegram chat. Navidrome keeps doing the playback.

**Never self-hosted music?** You don't need Navidrome yet. On a fresh Linux server the installer sets up Navidrome, the bot, and HTTPS, the bot imports your streaming likes and playlists, and you listen in a phone app such as [Symfonium](https://symfonium.app/) or [Amperfy](https://github.com/BLeeEZ/amperfy).

## Features

- [Add music from Telegram.](https://lubaskinc0de.github.io/beatstash/using/uploading/) Forward a track or send files from your phone. The bot fills in the tags, skips duplicates, and files the music into artist and album folders. MP3, FLAC, M4A, OGG, Opus, and WAV work.
- [Bring your streaming collection.](https://lubaskinc0de.github.io/beatstash/import/sources/) Import your likes, saved albums, and playlists from a supported service. Afterwards the bot keeps syncing what you save there.
- [One server for your friends.](https://lubaskinc0de.github.io/beatstash/using/libraries/) Send an invite link, and each person gets a Navidrome account, a personal library, and a storage limit you set. Tracks they want to show the group go into a shared library.
- [Share music in any chat.](https://lubaskinc0de.github.io/beatstash/using/sharing/) Type `@your_music_bot np` to send what's playing, or search your library right inside a conversation.
- [Works with the Navidrome you have.](https://lubaskinc0de.github.io/beatstash/installation/existing-navidrome/) Your music stays in its folders, and you can find and share it from Telegram.
- [An installer that does the setup.](https://lubaskinc0de.github.io/beatstash/installation/new-server/) It connects an existing Navidrome or installs everything from scratch, HTTPS included.

## Import from streaming services

| Service | Status |
|---|---|
| Zvuk | Likes, saved albums, playlists, and sync. Needs a subscription. |
| Yandex Music, YouTube Music, Spotify | Planned |

Details are in [supported sources](https://lubaskinc0de.github.io/beatstash/import/sources/).

## Quick start

You need a Linux server (AMD64 or ARM64) with [Docker Compose](https://docs.docker.com/compose/), a bot token from [@BotFather](https://t.me/BotFather), and `api_id` and `api_hash` from [my.telegram.org](https://my.telegram.org). The full list is in [requirements](https://lubaskinc0de.github.io/beatstash/installation/requirements/).

Run the installer on the server. It walks you through Telegram setup and asks whether to connect your Navidrome or install a new one:

```sh
curl -fL https://github.com/lubaskinc0de/beatstash/releases/latest/download/install.sh -o install.sh
bash install.sh
```

Run it again in the same folder to finish an interrupted setup. Later, `bash install.sh upgrade` with a newer release's installer updates the installation, and `bash install.sh uninstall` removes it.

The [documentation](https://lubaskinc0de.github.io/beatstash/) covers both install paths step by step, everyday use, HTTPS, and updates. To see how beatstash fits next to Navidrome and [beets](https://beets.io/), read the [comparison](https://lubaskinc0de.github.io/beatstash/introduction/comparison/).

## Contributing

Bug reports and new music providers are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)
