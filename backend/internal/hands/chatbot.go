package hands

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"healthtrack/internal/config"
	"healthtrack/internal/services/ai"
)

// missingKey reports whether an ai error means the backend is missing its
// configured API key. The ai package returns plain errors.New("no-api-key")
// and errors.New("tts-key-missing") values (not a shared sentinel), so
// errors.Is cannot detect them; fall back to matching the known strings.
func missingKey(err error, marker string) bool {
	return err != nil && strings.Contains(err.Error(), marker)
}

func chatbotAPI(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var b struct {
			Message string `json:"message"`
		}
		_ = c.ShouldBindJSON(&b)

		if b.Message == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No message provided"})
			return
		}

		text, err := ai.Chat(ctx, cfg, b.Message)
		if err != nil {
			if missingKey(err, "no-api-key") {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Server configuration error: API key missing"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Chat service error. Please try again later."})
			return
		}
		c.JSON(http.StatusOK, gin.H{"response": text})
	}
}

func chatbotTTS(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var b struct {
			Text string `json:"text"`
		}
		_ = c.ShouldBindJSON(&b)

		// The legacy JS rejects blank/whitespace-only text.
		if strings.TrimSpace(b.Text) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No text provided"})
			return
		}

		audioURI, err := ai.TTS(ctx, cfg, b.Text)
		if err != nil {
			if missingKey(err, "tts-key-missing") {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "TTS configuration error: API key missing"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "TTS service encountered an error."})
			return
		}
		c.JSON(http.StatusOK, gin.H{"audio": audioURI})
	}
}
