# Xiaomi MiMo ASR / TTS

This integration uses the existing ASR/TTS providers; no ESP32 firmware changes are required.

## Configuration

Select **小米 MiMo** in the manager's ASR and TTS settings. Set the API key, or set `MIMO_API_KEY` in the server environment. Both use `https://api.xiaomimimo.com/v1/chat/completions` by default.

- ASR provider: `mimo`, model: `mimo-v2.5-asr`, `sample_rate: 16000`, `language: auto|zh|en`. The adapter buffers each VAD-delimited utterance and uploads WAV; this is **not** realtime audio-input ASR.
- TTS provider: `mimo`, model: `mimo-v2.5-tts`, voice: `mimo_default` (or `冰糖`, `茉莉`, `苏打`, `白桦`, `Mia`, `Chloe`, `Milo`, `Dean`). The server decodes streamed 24 kHz PCM16 and reuses its existing Opus encoder.
- VoiceDesign: select `mimo-v2.5-tts-voicedesign`, enter a `voice_prompt` describing the target voice.
- VoiceClone: select `mimo-v2.5-tts-voiceclone` and set `reference_audio_path` to an **existing server-side WAV/MP3 file**. It is sent with each synthesis request; it is not a permanent remote voice ID.

VoiceDesign and VoiceClone currently use streaming-compatible responses but may only output audio once inference completes. This integration does not change the manager's existing voice-clone upload/asset workflow.

No Xiaomi-specific dependencies, dynamic plugin framework, or database migration are introduced.

Official API: https://mimo.mi.com/docs/zh-CN/quick-start/usage-guide/multimodal-understanding/speech-synthesis-v2.5

## Local smoke test

Use the feature branch; the fork's `main` does not contain MiMo yet.

```bash
git clone -b feature/xiaomi-mimo https://github.com/z7ping/xiaozhi-esp32-server-golang.git
cd xiaozhi-esp32-server-golang
git submodule update --init --recursive
go test ./pkg/mimo
go build -o xiaozhi_server ./cmd/server/
# Set MIMO_API_KEY in the environment before starting the server:
./xiaozhi_server -c config/config.yaml
```

The project requires Go 1.24+ as declared in go.mod, plus its native audio dependencies. On Windows use the corresponding `xiaozhi_server.exe` binary and set the environment variable with your shell.

Acceptance checklist: ASR recognizes a short Chinese sentence, standard TTS plays on ESP32, interruption works, and invalid API keys produce an error/empty-audio indication. VoiceDesign/VoiceClone are optional follow-on tests and can be slower. Do not paste an API key into issue/PR comments or logs.
