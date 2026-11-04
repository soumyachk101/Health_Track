package ai

import (
	"context"
	"strings"
	"testing"

	"healthtrack/internal/config"
)

// TestChatNoKey pins the deterministic error sentinel the chatbot handler
// keys on when neither Groq nor OpenRouter is usable. The Groq URL is
// hardcoded, so we only exercise the no-network error paths here: both keys
// empty, or the Groq placeholder falling through with an empty Key.
func TestChatNoKey(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		cfg  *config.Config
	}{
		{"both keys empty", &config.Config{GroqKey: "", Key: ""}},
		{"groq placeholder, no openrouter key", &config.Config{GroqKey: "your_groq_api_key_here", Key: ""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Chat(ctx, c.cfg, "hi")
			if err == nil {
				t.Fatal("Chat() error = nil, want no-api-key sentinel")
			}
			if !strings.Contains(err.Error(), "no-api-key") {
				t.Errorf("Chat() error = %q, want it to contain %q", err, "no-api-key")
			}
		})
	}
}

// TestTTSPlaceholder pins the TTS key-missing sentinel for both an empty and
// a placeholder Sarvam key. No network is involved on this path.
func TestTTSPlaceholder(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name  string
		key   string
	}{
		{"empty sarvam key", ""},
		{"placeholder sarvam key", "your_sarvam_api_key_here"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := TTS(ctx, &config.Config{SarvamKey: c.key}, "hello")
			if err == nil {
				t.Fatal("TTS() error = nil, want tts-key-missing sentinel")
			}
			if !strings.Contains(err.Error(), "tts-key-missing") {
				t.Errorf("TTS() error = %q, want it to contain %q", err, "tts-key-missing")
			}
		})
	}
}
