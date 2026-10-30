package chat

import (
	"strings"
	"testing"

	mcp_go "github.com/mark3labs/mcp-go/mcp"
)

func TestMCPHTTPAudioResourceLinkUsesExistingHTTPPlayer(t *testing.T) {
	link := mcp_go.ResourceLink{
		URI:      "http://media.example.local:8080/audio/temporary-token",
		Name:     "Test Song",
		MIMEType: "audio/mpeg",
	}
	// HTTP links do not require a resource/read MCP transport.
	source, err := buildMediaSourceFromResourceLink(link, nil)
	if err != nil {
		t.Fatalf("build HTTP audio source: %v", err)
	}
	if source.SourceType != MediaSourceTypeHTTPURL || source.HTTP == nil {
		t.Fatalf("want HTTP media source, got %+v", source)
	}
	if source.HTTP.URL != link.URI || source.Title != link.Name {
		t.Fatalf("unexpected link conversion: %+v", source)
	}
}

func TestMCPHTTPResourceLinkRejectsNonAudioContent(t *testing.T) {
	_, err := buildMediaSourceFromResourceLink(mcp_go.ResourceLink{
		URI:      "https://media.example.local/other",
		Name:     "Not Music",
		MIMEType: "text/html",
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "不是音频") {
		t.Fatalf("non-audio HTTP link should be rejected: %v", err)
	}
}

func TestMCPResourceLinkRejectsURLUserInfo(t *testing.T) {
	_, err := buildMediaSourceFromResourceLink(mcp_go.ResourceLink{
		URI:      "https://username:password@media.example.local/audio",
		Name:     "Music",
		MIMEType: "audio/mpeg",
	}, nil)
	if err == nil {
		t.Fatal("URL with embedded credentials must be rejected")
	}
}
