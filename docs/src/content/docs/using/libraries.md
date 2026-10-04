---
title: Libraries and quotas
description: Who sees each library, what goes where, and what counts toward your storage limit.
---

Everyone on the server keeps their own music apart and picks what to show the group. That works through three kinds of library.

| Library | What's in it | Who sees it |
|---|---|---|
| Personal | Your uploads, imports, and tracks you took from the shared library | You and the server's administrator |
| Shared | Tracks participants chose to share | Every participant |
| Existing | A Navidrome collection that was there before the bot | Accounts Navidrome lets in |

An invite doesn't give anyone access to your personal library. A track reaches the shared library only when someone shares it. Navidrome administrator accounts see every library.

Existing libraries stay in their original folders. The bot refreshes its records after Navidrome scans changes and never edits or deletes those files.

## What a quota measures

Your quota is the total size of the tracks in your personal library. Tracks you took from the shared library count in full, even though on disk they share space with the original. The shared library has its own quota for the whole server. Existing libraries count toward neither.

When your quota is full, new uploads and imports don't fit. When the shared quota is full, nobody can share more tracks. If your space is limited, the home screen shows how much you've used. To get more, ask the administrator; they [set quotas](../administration/participants.mdx#set-quotas) in the bot.
