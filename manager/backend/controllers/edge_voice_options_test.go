package controllers

import (
	"testing"

	edgetypes "github.com/difyz9/edge-tts-go/pkg/types"
)

func TestEdgeVoiceOptionsIncludesChineseDialects(t *testing.T) {
	voices := []edgetypes.Voice{
		{ShortName: "zh-CN-XiaoxiaoNeural", Locale: "zh-CN"},
		{ShortName: "zh-CN-shaanxi-XiaoniNeural", Locale: "zh-CN-shaanxi"},
		{ShortName: "en-US-JennyNeural", Locale: "en-US"},
	}
	options := edgeVoiceOptions(voices)
	if len(options) != 2 || options[0].Value != "zh-CN-XiaoxiaoNeural" || options[1].Value != "zh-CN-shaanxi-XiaoniNeural" {
		t.Fatalf("unexpected Edge voice options: %+v", options)
	}
}
