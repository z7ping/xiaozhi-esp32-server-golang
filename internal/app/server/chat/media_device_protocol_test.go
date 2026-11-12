package chat

import (
    "encoding/json"
    "testing"
)

func TestValidDeviceMediaAction(t *testing.T) {
    for _, action := range []string{"get_state", "resume", "pause", "stop", "prev", "next"} {
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
