# beatstash

An easy way to leave streaming behind and build your own music library.

[![Coverage](https://codecov.io/gh/lubaskinc0de/beatstash/branch/master/graph/badge.svg)](https://codecov.io/gh/lubaskinc0de/beatstash)
[![Stars](https://img.shields.io/github/stars/lubaskinc0de/beatstash?style=flat)](https://github.com/lubaskinc0de/beatstash/stargazers)
[![License: MIT](https://shieldcn.dev/github/lubaskinc0de/beatstash/license.svg?variant=secondary&font=geist&size=xs&mode=light)](LICENSE)

beatstash is a self-hosted Telegram bot for building and looking after a Navidrome music library. Send it music you want to keep, bring over your streaming collection, and share tracks with friends. The files live on your server, and you listen through Navidrome or your usual compatible app.

- [Save music from Telegram](https://lubaskinc0de.github.io/beatstash/using/uploading/). Forward a track from a conversation or send files from your phone, and the bot adds them to your library. It fills in available track details and checks for duplicates. MP3, FLAC, M4A, OGG, Opus, and WAV are supported.
- [Bring your saved music with you](https://lubaskinc0de.github.io/beatstash/import/sources/). Import a collection from a supported streaming service, then keep it up to date with periodic sync. Current provider support is listed below.
- [Invite friends to your server](https://lubaskinc0de.github.io/beatstash/using/libraries/). Each person gets a personal library and chooses what to add to the shared collection. You can set storage limits for each person from the bot.
- [Share what you are listening to](https://lubaskinc0de.github.io/beatstash/using/sharing/). Type `@your_music_bot np` in a chat, or search by artist or title. Send a track as Telegram audio or a public listening link, so friends can hear your recommendation.
- [Use the Navidrome collection you already have](https://lubaskinc0de.github.io/beatstash/installation/existing-navidrome/). Find and share existing music through Telegram while its files stay in their current folders.

Navidrome gives your music a home on your own server and makes it available in a web player and compatible apps. Keeping that collection growing takes more work: transferring new files to the server, finding a way to import saved music, and setting up accounts and library access for friends. Those jobs can interrupt something as simple as wanting to keep a song someone just sent you.

With beatstash, you can forward that song to the bot and listen to it in Navidrome. To recommend a song to friends, type your bot's username followed by the artist or title in any Telegram chat. Select the track, and the bot sends the audio or a listening link to that chat.

## Keep your music, keep adding to it

If you already have a Navidrome server, connect beatstash to add music and share your existing songs through Telegram. Your files stay in their current folders, and you keep listening in the same apps. Friends can search the libraries they have access to in Navidrome, and music they send to the bot goes into their own library.

If you want to move from streaming to your own music library, beatstash can bring over the music you have already saved on a supported service. You can keep adding music through Telegram and sharing it with friends. Downloaded music stays on your server even after you disconnect the streaming account.

| Streaming service | What you can use today |
|---|---|
| Zvuk | Import liked tracks, saved albums, and playlists; periodic sync after the first import. Requires an active subscription. |
| Yandex Music | Planned |
| YouTube Music | Planned |
| Spotify | Planned |

See [supported sources](https://lubaskinc0de.github.io/beatstash/import/sources/) for import details and requirements. You can upload audio files you already own regardless of which streaming service you use.

beatstash and Navidrome have no subscription fee. You provide the server and storage, and an import may still require a subscription to its source service.

## A music library you can share with friends

One server can hold several people's collections. Send a friend an invitation, and the bot helps them create or link a Navidrome account. They can upload music themselves, without running another server or asking you to handle each file.

Personal libraries stay separate. When a friend finds an album they want everyone to hear, they can add tracks to the shared library. You can listen through Navidrome, save a shared track to your own collection, or send it to another Telegram chat. The server becomes a place to exchange music as well as store it.

## Get started

Install with the ready-made [Docker image](https://github.com/lubaskinc0de/beatstash/pkgs/container/beatstash). Each [release](https://github.com/lubaskinc0de/beatstash/releases) includes Compose and configuration templates for Linux AMD64 and ARM64.

The interactive installer downloads the release files, guides you through Telegram setup, and saves the configuration. Run it on your server and choose whether to connect an existing Navidrome instance or install a new one:

```sh
curl -fL https://github.com/lubaskinc0de/beatstash/releases/latest/download/install.sh -o install.sh
bash install.sh
```

- [Already have Navidrome? Connect beatstash to it](https://lubaskinc0de.github.io/beatstash/installation/existing-navidrome/).
- [Starting fresh? Install Navidrome and beatstash together](https://lubaskinc0de.github.io/beatstash/installation/new-server/).

The [full documentation](https://lubaskinc0de.github.io/beatstash/) covers Telegram setup, everyday use, HTTPS, and updates. The [comparison with Navidrome and beets](https://lubaskinc0de.github.io/beatstash/introduction/comparison/) explains where each tool fits.

## Local development

Use Go 1.27.1, Docker Compose, `ffmpeg`, Bash, and `just`. Prepare `.env` and `config.toml` from the example files, configure your administrator and paths, then create the first Navidrome administrator before starting the bot:

```sh
docker compose up -d postgres navidrome
# Create the Navidrome administrator at http://localhost:4533.
just up
```

See [local development](https://lubaskinc0de.github.io/beatstash/development/local/) for the complete setup and lint-tool requirements. Checks:

```sh
just lint
just test
```

`just lint` runs the static checks for code, workflows, spelling, and documentation. `just test` runs release-policy tests and end-to-end scenarios. Install the tools listed in the [development guide](https://lubaskinc0de.github.io/beatstash/development/local/).

To preview the documentation with Node 24:

```sh
cd docs
npm ci
npm run dev
```

## Contributing

If a setup step is unclear or a track fails to import, a report with the steps and relevant logs helps us investigate. Contributions can also add a music provider or improve a guide. Read [CONTRIBUTING.md](CONTRIBUTING.md) for setup, checks, and pull-request guidance.

## License

[MIT](LICENSE)
