# beatstash

A self-hosted Telegram bot for managing your [Navidrome](https://www.navidrome.org/) music library.
And also the best tool for switching to self-hosted music.

[![CI](https://img.shields.io/github/actions/workflow/status/lubaskinc0de/beatstash/ci.yml?branch=master&label=CI)](https://github.com/lubaskinc0de/beatstash/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/lubaskinc0de/beatstash)](https://github.com/lubaskinc0de/beatstash/releases)
[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/lubaskinc0de/beatstash/badges/coverage.json)](https://github.com/lubaskinc0de/beatstash/actions/workflows/ci.yml?query=branch%3Amaster)
[![License: MIT](https://img.shields.io/github/license/lubaskinc0de/beatstash)](LICENSE)

<!-- TODO: GIF — forward a track to the bot, then share it with `@your_music_bot np` in a chat. -->

## Features
- [**Manage your library through telegram.**](https://lubaskinc0de.github.io/beatstash/using/sharing/) Forward a track or send files from your phone. The bot fills in track details and checks for duplicates, then, it neatly organizes the music on your server. 
Supports MP3, FLAC, M4A, OGG, Opus, and WAV.

- [**A single Navidrome instance for friends.**](https://lubaskinc0de.github.io/beatstash/using/libraries/) Invite people to your server. Each gets a personal library with a storage limit you set. Everyone can share their music with others via a shared collection, and you can compete to see who shares the most.

- [**Works with an existing Navidrome.**](https://lubaskinc0de.github.io/beatstash/installation/existing-navidrome/) Your music stays in its current folders, and you can find and share it through Telegram.

- [**Still using music streaming services and want to switch to self-hosted music?**](https://lubaskinc0de.github.io/beatstash/import/sources/#streaming-collections) A bot can help you set everything up and transfer your entire library with a single click, while also automatically syncing new albums and tracks from your connected accounts.

- [**Share your music in chats.**](https://lubaskinc0de.github.io/beatstash/using/sharing/) Invoke the bot directly in the any Telegram chat to share what you are currently listening to, your recently played tracks, or any track or album from your library.

- [**Guided setup.**](https://lubaskinc0de.github.io/beatstash/installation/new-server/) An interactive setup wizard will help you connect the bot to an existing Navidrome instance or set everything up from scratch, including HTTPS configuration and a fresh Navidrome installation.

## Music services you can import from
| Streaming service | Status |
|---|---|
| Zvuk | Liked tracks, albums, playlists, periodic sync. Requires a subscription. |
| Yandex Music | Planned |
| YouTube Music | Planned |
| Spotify | Planned |

See [supported sources](https://lubaskinc0de.github.io/beatstash/import/sources/) for details.

## Quick start

You need:

- a Linux server (AMD64 or ARM64) with [Docker Compose](https://docs.docker.com/compose/);
- a bot token from [@BotFather](https://t.me/BotFather);
- `api_id` and `api_hash` from [my.telegram.org](https://my.telegram.org), used by the local Bot API server for large files.

Run the installer on the server. It walks you through Telegram setup and asks whether to connect your existing Navidrome or install a new one:

```sh
curl -fL https://github.com/lubaskinc0de/beatstash/releases/latest/download/install.sh -o install.sh
bash install.sh
```

Run it again in the same directory to finish an interrupted setup. Later, `bash install.sh upgrade` with a newer release's installer updates the installation, and `bash install.sh uninstall` removes it.

Step-by-step guides: [connect to an existing Navidrome](https://lubaskinc0de.github.io/beatstash/installation/existing-navidrome/) or [set up a new server](https://lubaskinc0de.github.io/beatstash/installation/new-server/). The [documentation](https://lubaskinc0de.github.io/beatstash/) also covers everyday use, HTTPS, and updates. For how beatstash differs from Navidrome and [beets](https://beets.io/), see the [comparison](https://lubaskinc0de.github.io/beatstash/introduction/comparison/).

## Contributing

Bug reports and new music providers are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for local setup and checks.

## License

[MIT](LICENSE)
