# ES3C28P 触摸播放器控制协议（增量）

复用小智现有设备 WebSocket / MQTT 命令通道；音频仍沿用原 Opus/TTS 发送链路，Go 服务端的 MediaPlayer 是唯一播放状态源。

## 能力协商 / 向后兼容

新 Go 服务端在已有 `hello` 响应中返回：
```json
{"type":"hello","features":{"media_control":true}}
```
其余握手字段（`transport`、`session_id`、`audio_params`、MQTT 的 `udp`）仍按原协议返回。

ESP32 只有收到明确的布尔 `true` 才发送 `media` 控制。旧服务端未声明时自动关闭触摸控制和轮询，不影响传统 `listen`、`tts`、`mcp` 和原音频通道。断线和新 hello 会清空并重新解析能力。旧固件收到 `features` 这个可选字段可直接忽略。无会话/未握手时不能触发媒体控制。

## 设备 → 服务端

```json
{"type":"media","state":"get_state"}
{"type":"media","state":"get_playlist","index":0}
{"type":"media","state":"get_playlist","index":6}
{"type":"media","state":"select","index":2}
{"type":"media","state":"seek","position_ms":45000}
{"type":"media","state":"pause"}
{"type":"media","state":"resume"}
{"type":"media","state":"stop"}
{"type":"media","state":"prev"}
{"type":"media","state":"next"}
```

仅允许固定动作，绑定本设备的 ChatSession，不允许在消息中指定外部音频地址。触摸控制复用现有 SessionMediaPlayer、音频输出和控制通道；切歌、Seek、暂停等操作异步执行，不阻塞消息接收循环。

## 服务端 → 设备

```json
{
  "type": "media",
  "state": "playing",
  "title": "真实歌曲标题",
  "position_ms": 12500,
  "duration_ms": 212000,
  "current_index": 2,
  "playlist_length": 14,
  "page_offset": 0,
  "playlist": [
    {"index":0,"title":"第一首"},
    {"index":1,"title":"第二首"}
  ]
}
```

- 普通状态回复不附加 `page_offset` / `playlist`。列表响应最多返回 6 首，支持分页，不向 ESP32 发送音频资源、Jellyfin 凭证或文件路径。
- `duration_ms` 来自 MP3 解码器可确认的总样本数，或媒体源明确携带的 `duration_ms` 元数据。**无法确定时为 0，固件禁用 Seek 滑杆，不推算虚假时长**。
- Seek 在 **Go 服务端**重启当前曲目、从头解码并丢弃目标位置前的帧，再继续给 ESP32 推送原有音频包。最多允许两小时位置；远程或长音频可能等待较久，超出结尾返回错误。该通用实现不是随机字节范围 HTTP Seek。
- ESP32 只在播放器页面可见时每 5 秒查询一次播放状态；列表仅在用户打开或翻页时请求。
- 当前缺真机资源、音频流与 WebSocket/MQTT 联调，不能声称已交付。

## 测试

```bash
go test ./internal/app/server/chat -run 'Test(ValidDeviceMediaAction|DeviceMediaStateWireFormat|DeviceMediaPlaylistPageIsBounded|DeviceMediaPlaylistPageKeepsUTF8|SeekRequiresValidPositionAndQueue|DurationIsUpdatedOnlyForCurrentPlayback) -count=1
```

需要实机同时验证 WebSocket/MQTT、音频连续播放、暂停队列清空、恢复、上一首/下一首、断线后的错误显示。当前只提交源码和测试，没有宣称完成实际部署或 ESP32 联调。
 -count=1
```

需要实机同时验证 WebSocket/MQTT、音频连续播放、暂停队列清空、恢复、上一首/下一首、断线后的错误显示。当前只提交源码和测试，没有宣称完成实际部署或 ESP32 联调。
