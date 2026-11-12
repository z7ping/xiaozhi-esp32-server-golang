package chat

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"

    log "xiaozhi-esp32-server-golang/logger"
)

// Device media controls run on the existing authenticated device channel.
// This wire format carries only a bounded state summary, never media bytes.
type deviceMediaStateMessage struct {
    Type           string `json:"type"`
    State          string `json:"state"`
    Title          string `json:"title,omitempty"`
    PositionMs     int64  `json:"position_ms"`
    CurrentIndex   int    `json:"current_index"`
    PlaylistLength int    `json:"playlist_length"`
    Error          string `json:"error,omitempty"`
}

func validDeviceMediaAction(action string) bool {
    switch action {
    case "get_state", "resume", "pause", "stop", "prev", "next":
        return true
    }
    return false
}

func (c *ChatManager) HandleDeviceMediaMessage(action string) error {
    action = strings.ToLower(strings.TrimSpace(action))
    if !validDeviceMediaAction(action) {
        return fmt.Errorf("unsupported device media action: %q", action)
    }
    if c == nil || c.serverTransport == nil || c.managerClosing.Load() {
        return fmt.Errorf("device transport not ready")
    }
    session := c.GetSession()
    if session == nil || session.mediaPlayer == nil {
        return c.sendDeviceMediaState(nil, "player not ready")
    }

    if action == "get_state" {
        return c.sendDeviceMediaState(session, "")
    }

    // A pause/skip/stop may flush queued audio; do not block the receive loop.
    go func() {
        ctx := c.ctx
        if ctx == nil {
            ctx = context.Background()
        }
        _, err := controlMusicPlayback(ctx, session, &MusicPlaybackControlParams{Action: action})
        message := ""
        if err != nil {
            message = err.Error()
            log.Warnf("设备 %s 触摸媒体控制 %s 失败: %v", c.DeviceID, action, err)
        }
        if err := c.sendDeviceMediaState(session, message); err != nil {
            log.Debugf("设备 %s 回传媒体状态失败: %v", c.DeviceID, err)
        }
    }()
    return nil
}

func (c *ChatManager) sendDeviceMediaState(session *ChatSession, errMessage string) error {
    reply := deviceMediaStateMessage{
        Type: "media",
        State: "idle",
        CurrentIndex: -1,
        Error: errMessage,
    }
    if session != nil && session.mediaPlayer != nil {
        state := session.mediaPlayer.GetState()
        reply.State = state.Status.String()
        reply.Title = state.CurrentTitle
        reply.PositionMs = state.PositionMs
        reply.CurrentIndex = state.CurrentIndex
        reply.PlaylistLength = len(state.Playlist)
    }
    payload, err := json.Marshal(reply)
    if err != nil {
        return err
    }
    return c.serverTransport.SendCmd(payload)
}
