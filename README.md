# beatstash

A self-hosted Telegram bot for your [Navidrome](https://www.navidrome.org/) music library.

[![CI](https://img.shields.io/github/actions/workflow/status/lubaskinc0de/beatstash/ci.yml?branch=master&label=CI)](https://github.com/lubaskinc0de/beatstash/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/lubaskinc0de/beatstash)](https://github.com/lubaskinc0de/beatstash/releases)
[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/lubaskinc0de/beatstash/badges/coverage.json)](https://github.com/lubaskinc0de/beatstash/actions/workflows/ci.yml?query=branch%3Amaster)
[![License: MIT](https://img.shields.io/github/license/lubaskinc0de/beatstash)](LICENSE)

<!-- TODO: GIF — forward a track to the bot, then share it with `@your_music_bot np` in a chat. -->

Forward a track to the bot and it lands in your library. Listen in Navidrome or any Subsonic-compatible app. To recommend a song, type the bot's username in any chat and pick the track. The files stay on your server.

## Features

- **Save from Telegram.** Forward a track or send files from your phone. The bot fills in track details and checks for duplicates. Supports MP3, FLAC, M4A, OGG, Opus, and WAV.
- **Import from streaming.** Bring over liked tracks, albums, and playlists, then keep them in sync. Downloaded music stays after you disconnect the account.
- **Libraries for friends.** Invite people to your server. Each gets a personal library with a storage limit you set and can add tracks to a shared collection.
- **Share in any chat.** Type `@your_music_bot np` or search by artist or title, then send the track as audio or as a listening link.
- **Works with an existing Navidrome.** Your music stays in its current folders, and you can find and share it through Telegram.
- **Guided setup.** One command on the server asks a few questions and starts everything with Docker. It creates the Navidrome administrator for you. It can also put Navidrome behind HTTPS, either in the Caddy that already serves your other sites or in a Caddy of its own. Where Telegram is blocked, it sends the bot through your proxy. Every change is shown before it is made. The same tool updates the installation and removes it, asking about each part.

| Streaming service | Status |
|---|---|
| Zvuk | Liked tracks, albums, playlists, periodic sync. Requires a subscription. |
| Yandex Music | Planned |
| YouTube Music | Planned |
| Spotify | Planned |

See [supported sources](https://lubaskinc0de.github.io/beatstash/import/sources/) for details.

## Quick start

You need:

- a Linux server (AMD64 or ARM64) with Docker Compose;
- a bot token from [@BotFather](https://t.me/BotFather);
- `api_id` and `api_hash` from [my.telegram.org](https://my.telegram.org), used by the local Bot API server for large files.

Run the installer on the server. It walks you through Telegram setup and asks whether to connect your existing Navidrome or install a new one:

```sh
curl -fL https://github.com/lubaskinc0de/beatstash/releases/latest/download/install.sh -o install.sh
bash install.sh
```

Run it again in the same directory to finish an interrupted setup. Later, `bash install.sh upgrade` with a newer release's installer updates the installation, and `bash install.sh uninstall` removes it.

Step-by-step guides: [connect to an existing Navidrome](https://lubaskinc0de.github.io/beatstash/installation/existing-navidrome/) or [set up a new server](https://lubaskinc0de.github.io/beatstash/installation/new-server/). The [documentation](https://lubaskinc0de.github.io/beatstash/) also covers everyday use, HTTPS, and updates. For how beatstash differs from Navidrome and beets, see the [comparison](https://lubaskinc0de.github.io/beatstash/introduction/comparison/).

## Contributing

Bug reports and new music providers are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for local setup and checks.

## License

[MIT](LICENSE)
