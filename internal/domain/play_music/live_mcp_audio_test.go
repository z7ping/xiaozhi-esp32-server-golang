package play_music

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestLiveMCPAudioDecode(t *testing.T) {
	path := os.Getenv("MCP_AUDIO_URLS_FILE")
	if path == "" {
		t.Skip("live MCP URLs not configured")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var urls []string
	if err := json.Unmarshal(data, &urls); err != nil {
		t.Fatal(err)
	}
	if len(urls) < 2 {
		t.Fatal("need two tracks to check sequential decoding")
	}
	for i, audioURL := range urls[:2] {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		frames, err := PlayMusicStream(ctx, audioURL, 16000, 20, "mp3")
		if err != nil {
			cancel()
			t.Fatalf("track %d: %v", i+1, err)
		}
		select {
		case frame, ok := <-frames:
			if !ok || len(frame) == 0 {
				t.Fatalf("track %d returned no decoded audio", i+1)
			}
		case <-ctx.Done():
			t.Fatalf("track %d did not decode in time", i+1)
		}
		cancel()
	}
}
