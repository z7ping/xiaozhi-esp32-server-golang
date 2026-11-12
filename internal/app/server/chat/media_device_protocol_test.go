package chat

import (
    "encoding/json"
    "context"
    "testing"
    "strings"
    "unicode/utf8"

    "xiaozhi-esp32-server-golang/internal/domain/play_music"
)

func TestValidDeviceMediaAction(t *testing.T) {
    for _, action := range []string{"get_state", "get_playlist", "select", "seek", "resume", "pause", "stop", "prev", "next"} {
        if !validDeviceMediaAction(action) {
            t.Fatalf("%s should be accepted", action)
        }
    }
    for _, action := range []string{"", "play_playlist", "delete", "run"} {
        if validDeviceMediaAction(action) {
            t.Fatalf("%s should be rejected", action)
        }
    }
}

func TestDeviceMediaStateWireFormat(t *testing.T) {
    state := deviceMediaStateMessage{
        Type: "media", State: "playing", Title: "测试歌曲",
        PositionMs: 1234, CurrentIndex: 0, PlaylistLength: 2,
    }
    raw, err := json.Marshal(state)
    if err != nil { t.Fatal(err) }
    var got map[string]any
    if err := json.Unmarshal(raw, &got); err != nil { t.Fatal(err) }
    if got["state"] != "playing" || got["title"] != "测试歌曲" {
        t.Fatalf("unexpected wire state: %s", raw)
    }
    if got["position_ms"] != float64(1234) {
        t.Fatalf("position not encoded: %s", raw)
    }
}

func TestDeviceMediaPlaylistPageIsBounded(t *testing.T) {
    items := make([]deviceMediaQueueSummary, 19)
    for index := range items {
        items[index].Title = "第九首测试音乐"
    }
    page := deviceMediaPage(items, 6)
    if len(page) != 6 || page[0].Index != 6 || page[5].Index != 11 {
        t.Fatalf("unexpected page: %+v", page)
    }
    last := deviceMediaPage(items, 18)
    if len(last) != 1 || last[0].Index != 18 {
        t.Fatalf("unexpected last page: %+v", last)
    }
    if out := deviceMediaPage(items, 19); len(out) != 0 {
        t.Fatalf("out-of-range page should be empty: %+v", out)
    }
}

func TestDeviceMediaPlaylistPageKeepsUTF8(t *testing.T) {
    items := []deviceMediaQueueSummary{{
        Title: "演奏" + strings.Repeat("乐", 60),
    }}
    page := deviceMediaPage(items, 0)
    if len([]rune(page[0].Title)) != 48 || !utf8.ValidString(page[0].Title) {
        t.Fatalf("UTF-8 page title invalid: %+v", page[0])
    }
}

func TestSeekRequiresValidPositionAndQueue(t *testing.T) {
    coordinator := newMediaPlaybackCoordinator()
    runtime := coordinator.getOrCreateRuntime("seek-test")
    for _, target := range []int64{-1, 2*60*60*1000 + 1, 0} {
        if err := runtime.Seek(context.Background(), target, mediaPlaybackAudioConfig{}); err == nil {
            t.Fatalf("seek %d without queue should fail", target)
        }
    }
}

func TestDurationIsUpdatedOnlyForCurrentPlayback(t *testing.T) {
    coordinator := newMediaPlaybackCoordinator()
    runtime := coordinator.getOrCreateRuntime("duration-test")
    current := newActiveMediaPlayback(context.Background())
    old := newActiveMediaPlayback(context.Background())
    defer current.cancel()
    defer old.cancel()
    runtime.mu.Lock()
    runtime.active = current
    runtime.mu.Unlock()
    runtime.setDecodedDuration(old, 12000)
    if runtime.GetState().DurationMs != 0 {
        t.Fatal("stale decoder must not update duration")
    }
    runtime.setDecodedDuration(current, 12000)
    if runtime.GetState().DurationMs != 12000 {
        t.Fatal("current decoder duration was lost")
    }
}

func TestDeviceMediaSnapshotDoesNotCopyInlineAudioBytes(t *testing.T) {
    coordinator := newMediaPlaybackCoordinator()
    runtime := coordinator.getOrCreateRuntime("metadata-only-device")
    runtime.mu.Lock()
    runtime.mode = mediaPlaybackModeStandalone
    runtime.playbackIndex = 0
    runtime.state.CurrentTitle = "音频片段"
    runtime.standaloneItems = []MediaPlaylistItem{{
        Source: MediaSourceDescriptor{
            Title: "音频片段",
            Inline: &InlineAudioSource{Data: make([]byte, 1024*1024)},
        },
    }}
    runtime.mu.Unlock()
    player := &SessionMediaPlayer{runtime: runtime}
    state, queue, index := player.GetDeviceMediaSnapshot()
    if index != 0 || len(queue) != 1 || queue[0].Title != "音频片段" {
        t.Fatalf("wrong device queue metadata: %d %+v", index, queue)
    }
    if len(state.Playlist) != 0 {
        t.Fatal("device snapshot must not carry full media descriptors")
    }
}

func TestSeekRejectsKnownOutOfBoundsWithoutChangingPlayback(t *testing.T) {
    coordinator := newMediaPlaybackCoordinator()
    runtime := coordinator.getOrCreateRuntime("seek-bounds-test")
    current := newActiveMediaPlayback(context.Background())
    defer current.cancel()
    runtime.mu.Lock()
    runtime.active = current
    runtime.mode = mediaPlaybackModeStandalone
    runtime.playbackIndex = 0
    runtime.state.DurationMs = 1000
    runtime.state.Status = play_music.StatusPlaying
    runtime.standaloneItems = []MediaPlaylistItem{{Source: MediaSourceDescriptor{Title: "short"}}}
    runtime.mu.Unlock()

    err := runtime.Seek(context.Background(), 1500, mediaPlaybackAudioConfig{})
    if err == nil { t.Fatal("seek past known duration must fail") }
    runtime.mu.RLock()
    unchanged := runtime.active == current && runtime.state.DurationMs == 1000
    runtime.mu.RUnlock()
    if !unchanged { t.Fatal("invalid seek must not restart playback") }
}

func TestSeekPreservesPausedStatus(t *testing.T) {
    coordinator := newMediaPlaybackCoordinator()
    runtime := coordinator.getOrCreateRuntime("seek-paused-test")
    old := newActiveMediaPlayback(context.Background())
    old.closeDone()
    defer old.cancel()
    runtime.mu.Lock()
    runtime.active = old
    runtime.mode = mediaPlaybackModeStandalone
    runtime.playbackIndex = 0
    runtime.state.DurationMs = 120000
    runtime.state.Status = play_music.StatusPaused
    runtime.standaloneItems = []MediaPlaylistItem{{Source: MediaSourceDescriptor{
        Title: "paused-song", SourceType: MediaSourceTypeHTTPURL,
        HTTP: &HTTPMediaSource{URL: "https://example.com/audio.mp3"},
    }}}
    runtime.mu.Unlock()

    if err := runtime.Seek(context.Background(), 5000, mediaPlaybackAudioConfig{}); err != nil {
        t.Fatalf("paused seek failed: %v", err)
    }
    state := runtime.GetState()
    if state.Status != play_music.StatusPaused || state.PositionMs != 5000 ||
       state.DurationMs != 120000 {
        t.Fatalf("paused seek must keep status, position and duration: %+v", state)
    }
    stopRuntimeAndWait(t, runtime)
}

func TestMediaControlRejectsMissingRequiredArguments(t *testing.T) {
    manager := &ChatManager{serverTransport: &ServerTransport{}}
    invalid := int(-1)
    tooLarge := int64(2*60*60*1000 + 1)
    for _, tc := range []struct{
        action string
        index *int
        position *int64
    }{
        {"select", nil, nil},
        {"get_playlist", nil, nil},
        {"get_playlist", &invalid, nil},
        {"seek", nil, nil},
        {"seek", nil, &tooLarge},
    } {
        if err := manager.HandleDeviceMediaMessage(tc.action, tc.index, tc.position); err == nil {
            t.Fatalf("%s: invalid arguments should be rejected", tc.action)
        }
    }
}
