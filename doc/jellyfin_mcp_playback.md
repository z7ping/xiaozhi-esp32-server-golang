# Jellyfin MCP audio playback integration

This branch is **based on `feature/xiaomi-mimo` (PR #1)**; it does not modify Xiaomi providers.

## Overview

The Go server does **not** need a Jellyfin API key. Existing `ResourceLink` handling now distinguishes:

- `audio/*` links whose `uri` is `http(s)://...`: streamed directly using existing MediaPlayer HTTP audio decoding.
- Other MCP resource URIs: use the existing `resources/read` audio paging flow.

Multiple ResourceLinks returned by one tool invocation are queued in order via `SessionMediaPlayer.ReplacePlaylistAndPlayWithHandle`. Playback control remains `control_music_playback`; there is no Jellyfin-specific player branch.

## Jellyfin MCP setup

Use the modified `z7ping/jellyfin-mcp` `feature/mcp-audio-stream` branch and enable its opt-in HTTP audio endpoint. For example:

```sh
jellyfin-mcp --http --addr 0.0.0.0:8080 \
    --http-token YOUR_MCP_TOKEN \
    --audio-public-url http://MCP_HOST_REACHABLE_FROM_GO:8080 \
    --toolsets discovery,media,user,playback
```

Configure the Jellyfin URL and API key **only on Jellyfin MCP**. Connect the existing Go MCP client to its `/mcp` endpoint, including the MCP bearer token as required by the deployment.

## Voice workflow

- For a named track: call `jellyfin_search` to obtain `item_id`, then call `jellyfin_audio` with `item_id`. The Go player plays the returned HTTP ResourceLink.
- For a playlist in read-only mode: call `jellyfin_search` with `type=Playlist` (an empty query lists visible playlists) to identify its ID, then call `jellyfin_audio` with `playlist_id`. The MCP server itself fetches the playlist in order; the LLM does not copy hundreds of track IDs.
- `control_music_playback` continues to handle pause / resume / next / previous / stop.

Note: `jellyfin_play` controls an existing Jellyfin client session, not the XiaoZhi speaker.

## Validation status

Implementation and regression tests committed. Full Go repository tests, actual Jellyfin transcoding, and physical ESP32 audio output have **not been verified** in this environment. Test an MP3 and a FLAC track against Jellyfin before production use. The Go player currently decodes MP3 streams, so the MCP proxy requests MP3 transcoding.
