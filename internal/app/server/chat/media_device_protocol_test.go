package chat

import (
    "encoding/json"
    "testing"
    "strings"
    "unicode/utf8"
)

func TestValidDeviceMediaAction(t *testing.T) {
    for _, action := range []string{"get_state", "get_playlist", "select", "resume", "pause", "stop", "prev", "next"} {
        if !validDeviceMediaAction(action) {
            t.Fatalf("%s should be accepted", action)
        }
    }
    for _, action := range []string{"seek", "", "play_playlist", "delete", "run"} {
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
    page := deviceMediaPage(items, 8)
    if len(page) != 8 || page[0].Index != 8 || page[7].Index != 15 {
        t.Fatalf("unexpected page: %+v", page)
    }
    last := deviceMediaPage(items, 16)
    if len(last) != 3 || last[2].Index != 18 {
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
