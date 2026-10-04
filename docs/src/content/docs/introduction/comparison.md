---
title: Navidrome, beatstash, and beets
description: What each tool does, and how many steps everyday music chores take with and without beatstash.
---

[Navidrome](https://www.navidrome.org/) plays the files on your server. beatstash gets music onto that server through Telegram, invites people, and shares tracks in chats. [beets](https://beets.io/) identifies releases, corrects tags, and organizes files. They do different jobs and work together.

| What you want to do | Navidrome on its own | With beatstash | beets |
|---|---|---|---|
| Add a track a friend sent in Telegram | Save the file and copy it to a music folder on the server | Forward it to the bot | Import a file you already have through the CLI |
| Bring over your streaming collection | Find the audio and add it yourself | Connect a [supported service](../import/sources.md) and start an import | Not supported |
| Invite a friend | Create an account, set library access, send the login | Send an invite link; the bot creates the account | Not supported |
| Keep everyone's music separate | Create a library per person and assign access by hand | Each participant gets a personal library automatically | Not supported |
| Limit how much music each friend adds | Watch server storage yourself | Set a quota in Telegram | Not supported |
| Share a track in a Telegram chat | Find it, create a listening link, paste it | Search inline or send `np` | Not supported |
| Listen in a browser or app | Web player and Subsonic clients | The same player and clients | Through a player or plugin |
| Fill in track details | Reads existing tags | [Fills in missing tags and artwork, then writes them to the file](../using/uploading.mdx#track-details-and-artwork) | Matches releases and corrects tags |
| Organize files into folders | Scans the folders you made | Files new music by artist and album | Configurable naming and layout |

## If you already run Navidrome

Without the bot, every new track and every new person is work for you. A friend sends an album, and you put the files on the server. Someone joins, and you create their account and pick the libraries they can see. As the group grows, so does the list of libraries and permissions to keep straight.

With beatstash, participants do this themselves. They accept an invite, upload through Telegram, and decide what to put in the shared library. The bot creates their libraries and sets access in Navidrome. You [set storage limits](../administration/participants.mdx#set-quotas) instead of handling each upload.

Navidrome still owns playback, accounts, and access control; the bot drives them through its API. Your existing libraries stay where they are, and who sees them is still decided in Navidrome. To add the bot to your server, follow the [connection guide](../installation/existing-navidrome.mdx). Navidrome's own features are described in its [overview](https://www.navidrome.org/docs/overview/) and [sharing documentation](https://www.navidrome.org/docs/usage/features/sharing/).

## If you are leaving a streaming service

A music server can't replace a streaming library until the audio files are on it. For supported services, beatstash downloads your collection: you connect an account, review what will be imported, and start. Later, sync brings in music you save there. Check the [source table](../import/sources.md) first, because only some services are supported.

## Where beets fits

Use beets when your files have inconsistent tags or folder names. Its import step identifies each release and fixes the metadata before organizing the files. Start with its [getting-started guide](https://docs.beets.io/en/stable/guides/main.html) and [plugins](https://docs.beets.io/en/stable/plugins/index.html).

All three can run together. Organize a collection with beets, add its folder as a Navidrome library, and connect beatstash to search and share it from Telegram. Keep the beets folder apart from the bot's [managed folders](../reference/glossary.md#managed-folders). Changes appear in the bot after Navidrome scans them and the bot refreshes its records.
