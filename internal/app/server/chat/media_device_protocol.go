package chat

import (
    "encoding/json"
    "fmt"
    "strings"
    "strconv"

    log "xiaozhi-esp32-server-golang/logger"
)

const deviceMediaPageSize = 6
const deviceMediaControlQueueCapacity = 8

type deviceMediaCommand struct {
    session *ChatSession
    action string
    index int
    positionMs int64
}

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

    // Device messages arrive on one receive loop. Enqueue in that order,
    // without spawning competing goroutines for rapid user taps.
    if c.ctx == nil {
        return fmt.Errorf("device session context unavailable")
    }
    cmd := deviceMediaCommand{session: session, action: action, index: -1, positionMs: -1}
    if index != nil { cmd.index = *index }
    if positionMs != nil { cmd.positionMs = *positionMs }
    c.mediaControlOnce.Do(func() {
        c.mediaControlQueue = make(chan deviceMediaCommand, deviceMediaControlQueueCapacity)
        go c.runDeviceMediaControls()
    })
    select {
    case c.mediaControlQueue <- cmd:
        return nil
    default:
        return fmt.Errorf("device media command queue full")
    }
}

func (c *ChatManager) runDeviceMediaControls() {
    ctx := c.ctx
    for {
        select {
        case <-ctx.Done():
            return
        case cmd := <-c.mediaControlQueue:
            if c.managerClosing.Load() {
                return
            }
            // Never execute commands from a previous device session.
            if c.GetSession() != cmd.session {
                continue
            }
            var err error
            switch cmd.action {
            case "select":
                err = cmd.session.mediaPlayer.PlayQueueIndex(ctx, cmd.index)
            case "seek":
                err = cmd.session.mediaPlayer.Seek(ctx, cmd.positionMs)
            default:
                _, err = controlMusicPlayback(ctx, cmd.session,
                    &MusicPlaybackControlParams{Action: cmd.action})
            }
            message := ""
            if err != nil {
                // Errors can contain private media URLs and MCP parameters.
                message = "媒体操作失败，请重试"
                log.Warnf("设备 %s 触摸媒体控制 %s 失败: %v", c.DeviceID, cmd.action, err)
            }
            offset := -1
            if cmd.action == "select" && err == nil {
                offset = (cmd.index / deviceMediaPageSize) * deviceMediaPageSize
            }
            if err := c.sendDeviceMediaState(cmd.session, message, offset); err != nil {
                log.Debugf("设备 %s 回传媒体状态失败: %v", c.DeviceID, err)
            }
        }
    }
}

func (c *ChatManager) sendDeviceMediaState(session *ChatSession, errMessage string, pageOffset int) error {
    reply := deviceMediaStateMessage{
        Type: "media", State: "idle", CurrentIndex: -1, Error: errMessage,
    }
    if session != nil && session.mediaPlayer != nil {
        state, items, index := session.mediaPlayer.GetDeviceMediaSnapshot()
        reply.State = state.Status.String()
        // Errors such as an overlong seek can occur asynchronously after the
        // command was accepted. Report a safe status on subsequent polls.
        if reply.Error == "" && state.ErrMsg != "" {
            reply.Error = "播放失败，请重新选择歌曲"
        }
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
