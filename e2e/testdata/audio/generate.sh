#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

ARTIST="Fixture Artist"
TITLE="Fixture Song"
ALBUM="Fixture Album"

tone() {
    local out=$1
    shift
    ffmpeg -hide_banner -loglevel error -y \
        -f lavfi -i "sine=frequency=440:duration=1.5" \
        -map_metadata -1 \
        -metadata artist="$ARTIST" \
        -metadata title="$TITLE" \
        -metadata album="$ALBUM" \
        -metadata track=1 \
        "$@" "$out"
}

tone track.mp3 -c:a libmp3lame -b:a 128k -id3v2_version 3
tone track.flac -c:a flac
tone track.m4a -c:a aac -b:a 128k
tone track.ogg -c:a libvorbis -q:a 3
tone track.opus -c:a libopus -b:a 64k
tone track.wav -c:a pcm_s16le
