# ES3C28P 触摸播放器控制协议（增量）

复用小智现有设备 WebSocket / MQTT 命令通道；音频仍沿用原 Opus/TTS 发送链路，Go 服务端的 MediaPlayer 是唯一播放状态源。

## 设备 → 服务端

```json
{"type":"media","state":"get_state"}
{"type":"media","state":"pause"}
{"type":"media","state":"resume"}
{"type":"media","state":"stop"}
{"type":"media","state":"prev"}
{"type":"media","state":"next"}
```

仅接受以上固定动作，复用与设备连接绑定的 ChatSession，不允许指定其他设备或播放来源。暂停、停止等可能等待音频队列清理的操作在独立 goroutine 执行，不阻塞收包循环。

## 服务端 → 设备

```json
{
  "type": "media",
  "state": "playing",
  "title": "当前歌曲名称",
  "position_ms": 12500,
  "current_index": 0,
  "playlist_length": 3
}
```

失败时在响应中增加 `error`；播放状态由既有 MediaPlayerState 获取，可能为 `idle / playing / paused / stopped / error`。未提供总时长，因此设备端暂时只显示已播放时间，不应假造 Seek 或总时长。

设备打开播放器时立即请求 `get_state`，页面可见期间每 5 秒轮询一次；不创建新的音频流、设备媒体缓存或独立连接。服务端处理控制后立即返回最新状态。进入播放器之外的正常语音播放仍不受影响。

## 测试

```bash
go test ./internal/app/server/chat -run 'Test(ValidDeviceMediaAction|DeviceMediaStateWireFormat)$' -count=1
```

需要实机同时验证 WebSocket/MQTT、音频连续播放、暂停队列清空、恢复、上一首/下一首、断线后的错误显示。当前只提交源码和测试，没有宣称完成实际部署或 ESP32 联调。
