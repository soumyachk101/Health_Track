// chatbot.go — the /chatbot AI surface (finding S3). Both routes run
// behind the auth module's AuthRequired + a per-user AI quota; the emergency
// branch is answered BEFORE any model call; the general-assistant instruction
// replaces the legacy "medical assistant" prompt; chart data reaches the
// model only when the client explicitly opts in (share_chart:true).
//
// [INPUT] depends on services/ai (chat chain + guard) and modules/auth
// (AuthRequired via the router + CheckAIQuota + CurrentUser);
// [OUTPUT] provides chatbotAPI / chatbotTTS gin handlers;
// [POS] the AI endpoint handlers of hands — policy lives in ai.guard, the
// gate + quota live in the auth module, this file only binds and dispatches;
// [PROTOCOL] Update this header on change, then check CLAUDE.md
package hands

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"healthtrack/internal/config"
	"healthtrack/internal/modules/auth"
	"healthtrack/internal/services/ai"
)

// maxMessageBytes is the 8 KB cap on a single chatbot message (finding S3):
// a huge payload must not flood the provider or the prompt window.
const maxMessageBytes = 8 * 1024

// chartBlockCap bounds how much opt-in chart context is appended to the
// prompt, independent of the 1 MB global body limit.
const chartBlockCap = 2048

// missingKey reports whether an ai error means the backend is missing its
// configured API key. The ai package returns plain errors.New("no-api-key")
// / ("tts-key-missing") values (not a shared sentinel), so match the strings.
func missingKey(err error, marker string) bool {
	return err != nil && strings.Contains(err.Error(), marker)
}

// chatRequest is the /chatbot/api body. share_chart defaults to false: when
// it is not explicitly true, NO chart data is sent to the model (ADR rule).
type chatRequest struct {
	Message    string `json:"message"`
	ShareChart bool   `json:"share_chart"`
	Chart      any    `json:"chart"`
}

func chatbotAPI(cfg *config.Config, authSvc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var b chatRequest
		if err := c.ShouldBindJSON(&b); err != nil {
			// Strict binding (S9): malformed/oversized JSON is a 400, not a
			// zero-value fall-through.
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		if strings.TrimSpace(b.Message) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No message provided"})
			return
		}
		if len(b.Message) > maxMessageBytes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Message too long"})
			return
		}

		// Emergency intent is handled BEFORE any model call and BEFORE the
		// quota: a safety reply must never be throttled or routed through a
		// provider that might diagnose.
		if ai.IsEmergencyIntent(b.Message) {
			c.JSON(http.StatusOK, gin.H{"response": ai.EmergencyReply(), "emergency": true})
			return
		}

		// Per-user quota (finding S3). The user is published by AuthRequired.
		u := auth.CurrentUser(c)
		if u != nil {
			allowed, qerr := authSvc.CheckAIQuota(ctx, u.ID, "chat")
			if qerr != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI service temporarily unavailable"})
				return
			}
			if !allowed {
				c.JSON(http.StatusTooManyRequests, gin.H{"error": "AI rate limit reached, try again later"})
				return
			}
		}

		// Chart context is included only on explicit opt-in, and only a
		// bounded slice of it. Default: none.
		prompt := b.Message
		if b.ShareChart && b.Chart != nil {
			if block, _ := json.Marshal(b.Chart); len(block) > 0 {
				if len(block) > chartBlockCap {
					block = block[:chartBlockCap]
				}
				prompt = "Health data the user chose to share (do not diagnose; give general information only):\n" +
					string(block) + "\n\nQuestion:\n" + b.Message
			}
		}

		text, err := ai.ChatWithInstruction(ctx, cfg, prompt, ai.AssistantInstruction)
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

// ttsRequest is the /chatbot/api/tts body.
type ttsRequest struct {
	Text string `json:"text"`
}

func chatbotTTS(cfg *config.Config, authSvc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var b ttsRequest
		if err := c.ShouldBindJSON(&b); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		// The legacy JS rejects blank/whitespace-only text.
		if strings.TrimSpace(b.Text) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No text provided"})
			return
		}
		if len(b.Text) > maxMessageBytes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Text too long"})
			return
		}

		// Per-user quota (finding S3).
		if u := auth.CurrentUser(c); u != nil {
			allowed, qerr := authSvc.CheckAIQuota(ctx, u.ID, "tts")
			if qerr != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "TTS service temporarily unavailable"})
				return
			}
			if !allowed {
				c.JSON(http.StatusTooManyRequests, gin.H{"error": "AI rate limit reached, try again later"})
				return
			}
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
