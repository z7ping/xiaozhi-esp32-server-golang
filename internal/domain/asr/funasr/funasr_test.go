package funasr

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestStreamingRecognizeCancelReleasesConnection(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	host, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewFunasr(FunasrConfig{Host: host, Port: port, Mode: "offline", SampleRate: 16000})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := provider.StreamingRecognize(ctx, make(chan []float32)); err != nil {
		t.Fatal(err)
	}
	cancel()

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_, err := provider.StreamingRecognize(ctx, make(chan []float32))
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("next recognition blocked after previous context was canceled")
	}
}
