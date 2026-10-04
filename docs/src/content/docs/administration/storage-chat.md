---
title: Storage chat
description: Let the bot send imported and existing tracks as audio instead of listening links.
---

Telegram can only send audio the bot has already uploaded to it. Tracks you upload through the bot are there from the start. Tracks imported from a streaming service, or read from an [existing library](../reference/glossary.md#existing-library), are not. When someone shares one of them with `np`, `recent`, or a search, the bot sends a [listening link](../reference/glossary.md#listening-link) instead.

A storage chat fixes that. It is a private channel where the bot uploads these files, so it can send them as audio afterwards.

## Set it up

1. Create a private channel in Telegram. Name it as you like; nobody else needs to join.
2. Open the channel info, then **Administrators > Add Admin**. Find your bot by its username and keep **Post Messages** allowed.
3. Open the channel in [Telegram Web](https://web.telegram.org/k/). The address ends with `#-100…`; that number, with the minus sign, is the channel ID.
4. Put the ID in the configuration on the server:

   ```toml title="deploy/config.toml"
   [telegram]
   storage_chat_id = -1001234567890
   ```

5. [Apply the change](./configuration.md#change-a-setting):

   ```sh
   docker compose up -d --force-recreate bot
   ```

## Upload in advance or on request

With `fill_storage_chat = true`, the bot uploads files to the channel in the background, so they are ready before anyone asks. With `false`, it uploads a file the first time someone picks it, and that person waits a little longer.

Leave `storage_chat_id` at `0` to go without a storage chat. Inline results for such tracks then fall back to [listening links](../installation/https.md) or to preparing the file after someone picks it.
