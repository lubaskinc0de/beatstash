---
title: Navidrome, beatstash, and beets
description: Compare the steps needed to add music, invite friends, share tracks, and organize your files.
---

[Navidrome](https://www.navidrome.org/) makes files on your server available to listen to. beatstash adds a Telegram workflow for getting music onto that server, inviting people, and sharing recommendations. [beets](https://beets.io/) helps identify releases, correct tags, and organize files.

| What you want to do | Navidrome on its own | With beatstash | beets |
|---|---|---|---|
| Add a track a friend sent in Telegram | Save the file and copy it to a server music folder yourself | Forward it to the bot; it checks and adds the track | Import a file you already have through the CLI |
| Bring over your saved streaming collection | Obtain the audio and add it yourself; no built-in collection downloader | Connect a supported source and start an import; Zvuk is available now | No built-in streaming collection download |
| Invite a friend | Create an account, set library access, and send them the login details | Send an invite link; the bot guides them through creating or linking an account | No server invitation workflow |
| Keep everyone's music separate | Set up separate libraries and assign each account's access yourself | The bot creates a personal library for each participant and a shared library for the group | Organizes files; does not manage Navidrome participants |
| Limit how much music each friend can add | Manage server storage yourself | Set a default or individual quota in Telegram | No participant quota workflow |
| Share music in a Telegram conversation | Find a track or create a public link, then send it yourself | Search in the chat, send a track, or share what's playing with `np` | No built-in Telegram chat workflow |
| Listen in a browser or phone app | Web player and compatible clients | Use the same Navidrome player or clients | Use a player or a playback plugin |
| Fill in track details | Reads the tags already in your files | [Adds available track details and import artwork, then writes them into the file](../using/uploading.md#track-details-and-artwork) | Identifies releases and helps correct tags |
| Organize files into folders | Scans the folders you set up | Places new music into artist and album folders automatically | Configurable file naming and folder organization |

## What changes for a Navidrome owner

With Navidrome alone, adding music and managing accounts are jobs for the server owner. If a friend sends you an album, you put its files on the server. If another person joins, you create their account and choose the libraries they can see. Keeping separate collections means maintaining those libraries and permissions as the group grows.

beatstash lets participants do more for themselves. They accept an invitation, upload their own files through Telegram, and decide what to publish to the group. The bot creates their libraries and assigns access. You can [set storage limits](../using/libraries.md#change-quotas-as-an-administrator) without changing folders or server settings for each upload.

Navidrome still provides playback, accounts, and library access controls. beatstash uses those capabilities to automate setup and bring everyday music management into Telegram. Existing libraries stay in place; access to them continues to depend on Navidrome permissions.

Read the [connection guide](../installation/existing-navidrome.md) to add the bot to a server you already use. For Navidrome's own features, see its [overview](https://www.navidrome.org/docs/overview/) and [sharing documentation](https://www.navidrome.org/docs/usage/features/sharing/).

## What changes when you leave streaming

A playback server needs audio files before it can replace your streaming library. beatstash provides an import path for supported sources: connect an account, review the collection, and start downloading. Later sync brings in new saved music. Check the [provider table](../import/sources.md) before planning your move: Zvuk is implemented; Yandex Music, YouTube Music, and Spotify are planned.

## Where beets helps

Use beets when you have audio files with inconsistent tags or folder names. Its import workflow identifies releases and helps correct metadata before organizing the files. See its [getting-started guide](https://docs.beets.io/en/stable/guides/main.html) and [plugins](https://docs.beets.io/en/stable/plugins/index.html).

You can use all three: organize a collection with beets, add its folder as a Navidrome library, and connect beatstash so you can search and share that music through Telegram. Keep the beets collection separate from the bot's managed folders. File changes become visible after Navidrome scans them and beatstash refreshes its records.
