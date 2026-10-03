---
title: Friends, libraries, and quotas
description: Who can see each library, how invitations work, and what counts toward storage limits.
---

## Invite a friend

Only beatstash administrators issue invitations. Open **Invite** on the home screen, copy the link, and send it to one person. Use **New invite** for another participant. Each link is single-use and expires after seven days by default.

An invited participant creates a Navidrome account or links an existing one. They receive their own personal library and access to the shared library. Access to an existing attached library can be granted in Navidrome.

## Library access

| Library | Contents | Who sees it |
|---|---|---|
| Personal | Your new uploads, imports, and tracks taken from the shared library | You and the server owner |
| Shared | Tracks participants deliberately publish | Server participants |
| Attached | A pre-existing Navidrome collection registered in the bot's database | Accounts allowed to access it in Navidrome |

Inviting someone does not give them access to your personal music. Publishing to the shared library is an explicit action. Navidrome administrator accounts can see all libraries.

Attached libraries remain on disk in their original folders. The bot refreshes their records after Navidrome scans changes. It does not edit or remove their original files.

## What a quota measures

A personal quota is the total size of tracks in your managed personal library. Tracks taken from the shared library count in full, even when the copies share disk storage through hardlinks. The shared library has a separate server-wide quota. Attached libraries do not count toward these quotas.

When your personal quota is full, new uploads and imports cannot fit. When the shared quota is full, publication stops. The home screen shows usage when your space is limited.

## Change quotas as an administrator

Open **Admin > Quotas** to change the default personal quota or shared quota. Open **Admin > Users**, choose a participant, then **Change the quota** for an individual limit.

You can enter values such as `25 GB` or `500 MB`, choose **Unlimited**, or revert to the default/configuration value. GB and MB are interpreted as binary units. Changing the default affects participants without an individual override.

The administrator screen shows promised capacity and free disk space. Quotas can promise more space than the disk has; they do not reserve physical storage.
