package chat

import (
    "encoding/json"
    "context"
    "testing"
    "strings"
    "unicode/utf8"
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
    items := make([]MediaPlaylistItem, 19)
    for index := range items {
        items[index].Source.Title = "第九首测试音乐"
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
    items := []MediaPlaylistItem{{Source: MediaSourceDescriptor{
        Title: "演奏" + strings.Repeat("乐", 60),
    }}}
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
