package chat

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "strconv"

    log "xiaozhi-esp32-server-golang/logger"
)

const deviceMediaPageSize = 6

// Device media controls run on the existing device command channel.
// Only bounded metadata is sent to ESP32, never audio or source URLs.
type deviceMediaPlaylistEntry struct {
    Index int    `json:"index"`
    Title string `json:"title"`
}

type deviceMediaStateMessage struct {
    Type           string                     `json:"type"`
    State          string                     `json:"state"`
    Title          string                     `json:"title,omitempty"`
    PositionMs     int64                      `json:"position_ms"`
    DurationMs     int64                      `json:"duration_ms"`
    CurrentIndex   int                        `json:"current_index"`
    PlaylistLength int                        `json:"playlist_length"`
    PageOffset     *int                       `json:"page_offset,omitempty"`
    Playlist       []deviceMediaPlaylistEntry `json:"playlist,omitempty"`
    Error          string                     `json:"error,omitempty"`
}

func validDeviceMediaAction(action string) bool {
    switch action {
    case "get_state", "get_playlist", "select", "seek", "resume", "pause", "stop", "prev", "next":
        return true
    }
    return false
}

func deviceMediaPage(items []deviceMediaQueueSummary, offset int) []deviceMediaPlaylistEntry {
    if offset < 0 || offset >= len(items) {
        return nil
    }
    end := offset + deviceMediaPageSize
    if end > len(items) {
        end = len(items)
    }
    page := make([]deviceMediaPlaylistEntry, 0, end-offset)
    for index := offset; index < end; index++ {
        title := []rune(items[index].Title)
        if len(title) > 48 {
            title = title[:48]
        }
        page = append(page, deviceMediaPlaylistEntry{
            Index: index, Title: string(title),
        })
    }
    return page
}

// The client is authenticated by the existing device session; it cannot pick a
// different playback runtime, audio URL or server-side file path.
func (c *ChatManager) HandleDeviceMediaMessage(action string, index *int, positionMs *int64) error {
    if c == nil || c.serverTransport == nil || c.managerClosing.Load() {
        return fmt.Errorf("device transport not ready")
    }
    action = strings.ToLower(strings.TrimSpace(action))
    if !validDeviceMediaAction(action) {
        return fmt.Errorf("unsupported device media action: %q", action)
    }
    if (action == "select" || action == "get_playlist") && (index == nil || *index < 0) {
        return fmt.Errorf("missing or invalid playlist index for %s", action)
    }
    if action == "seek" && (positionMs == nil || *positionMs < 0 || *positionMs > 2*60*60*1000) {
        return fmt.Errorf("missing or invalid seek position")
    }
    session := c.GetSession()
    if session == nil || session.mediaPlayer == nil {
        return c.sendDeviceMediaState(nil, "player not ready", -1)
    }
    if action == "get_state" {
        return c.sendDeviceMediaState(session, "", -1)
    }
    if action == "get_playlist" {
        return c.sendDeviceMediaState(session, "", *index)
    }

    // Take copies before the command is scheduled; never use decoded message
    // pointers in the goroutine after the original JSON is released.
    selectedIndex := -1
    if index != nil { selectedIndex = *index }
    targetMs := int64(-1)
    if positionMs != nil { targetMs = *positionMs }

    go func() {
        c.mediaControlMu.Lock()
        defer c.mediaControlMu.Unlock()
        if c.managerClosing.Load() { return }
        ctx := c.ctx
        if ctx == nil {
            ctx = context.Background()
        }
        var err error
        if action == "select" {
            err = session.mediaPlayer.PlayQueueIndex(ctx, selectedIndex)
        } else if action == "seek" {
            err = session.mediaPlayer.Seek(ctx, targetMs)
        } else {
            _, err = controlMusicPlayback(ctx, session, &MusicPlaybackControlParams{Action: action})
        }
        errMessage := ""
        if err != nil {
            // The full error may contain authenticated source URLs or MCP
            // resource addresses. Log server-side, send no secrets to device.
            errMessage = "媒体操作失败，请重试"
            log.Warnf("设备 %s 触摸媒体控制 %s 失败: %v", c.DeviceID, action, err)
        }
        pageOffset := -1
        if action == "select" && err == nil {
            pageOffset = (selectedIndex / deviceMediaPageSize) * deviceMediaPageSize
        }
        if err := c.sendDeviceMediaState(session, errMessage, pageOffset); err != nil {
            log.Debugf("设备 %s 回传媒体状态失败: %v", c.DeviceID, err)
        }
    }()
    return nil
}

func (c *ChatManager) sendDeviceMediaState(session *ChatSession, errMessage string, pageOffset int) error {
    reply := deviceMediaStateMessage{
        Type: "media", State: "idle", CurrentIndex: -1, Error: errMessage,
    }
    if session != nil && session.mediaPlayer != nil {
        state, items, index := session.mediaPlayer.GetDeviceMediaSnapshot()
        reply.State = state.Status.String()
        title := []rune(state.CurrentTitle)
        if len(title) > 64 { title = title[:64] }
        reply.Title = string(title)
        reply.PositionMs = state.PositionMs
        reply.DurationMs = state.DurationMs
        if reply.DurationMs == 0 && index >= 0 && index < len(items) {
            raw := strings.TrimSpace(items[index].DurationMs)
            if duration, err := strconv.ParseInt(raw, 10, 64); err == nil &&
               duration > 0 && duration <= 24*60*60*1000 {
                reply.DurationMs = duration
            }
        }
        reply.CurrentIndex = index
        reply.PlaylistLength = len(items)
        if pageOffset >= 0 {
            reply.PageOffset = &pageOffset
            reply.Playlist = deviceMediaPage(items, pageOffset)
        }
    }
    encoded, err := json.Marshal(reply)
    if err != nil {
        return err
    }
    return c.serverTransport.SendCmd(encoded)
}
