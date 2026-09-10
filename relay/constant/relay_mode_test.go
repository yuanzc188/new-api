package constant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPath2RelayMode(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{path: "/v1/alpha/search", want: RelayModeAlphaSearch},
		{path: "/v1/alpha/search?foo=1", want: RelayModeAlphaSearch},
		// 声音克隆必须与 /v1/audio/* 的其它前缀区分开，否则会被 AudioHelper 接管
		{path: "/v1/audio/voices", want: RelayModeAudioVoiceClone},
		{path: "/v1/audio/speech", want: RelayModeAudioSpeech},
		{path: "/v1/audio/transcriptions", want: RelayModeAudioTranscription},
		{path: "/v1/tts", want: RelayModeFishTTS},
		{path: "/v1/tts/stream/with-timestamp", want: RelayModeFishTTS},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, Path2RelayMode(tt.path))
		})
	}
}
