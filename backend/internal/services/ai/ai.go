// Package ai ports routes/chatbot.js: Groq -> OpenRouter chat fallback and
// Sarvam text-to-speech, using net/http only.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"healthtrack/internal/config"
)

// SystemInstruction is copied verbatim from routes/chatbot.js
// (note the trailing spaces on the first two lines, as in the JS source).
const SystemInstruction = "You are HealthTrack+ AI, a helpful and professional medical assistant. \n" +
	"Your goal is to help users understand their health data, provide general wellness advice, \n" +
	"and assist with navigating the HealthTrack+ platform.\n" +
	"Always include a disclaimer that you are an AI and not a substitute for professional medical advice."

const timeout = 30 * time.Second

var client = &http.Client{Timeout: timeout}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// Chat sends the message to Groq when configured, otherwise to OpenRouter.
// Returns an error "no-api-key" when neither fallback is available.
func Chat(ctx context.Context, cfg *config.Config, message string) (string, error) {
	msgs := []chatMessage{
		{Role: "system", Content: SystemInstruction},
		{Role: "user", Content: message},
	}

	if cfg.GroqKey != "" && cfg.GroqKey != "your_groq_api_key_here" {
		if content, err := postChat(ctx, "https://api.groq.com/openai/v1/chat/completions", "llama-3.3-70b-versatile", cfg.GroqKey, msgs); err == nil {
			return content, nil
		}
	}

	key := cfg.Key
	if key == "" {
		return "", errors.New("no-api-key")
	}

	// Mirrors routes/chatbot.js which POSTs to OpenRouter.
	content, err := postChat(ctx, "https://openrouter.ai/api/v1/chat/completions", "google/gemini-2.0-flash-001", key, msgs)
	if err != nil {
		return "", err
	}
	return content, nil
}

func postChat(ctx context.Context, url, model, bearer string, msgs []chatMessage) (string, error) {
	body, _ := json.Marshal(chatRequest{Model: model, Messages: msgs})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("OpenRouter API error: %s", string(raw))
	}
	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("OpenRouter API error: no choices")
	}
	return out.Choices[0].Message.Content, nil
}

type ttsRequest struct {
	Inputs              []string `json:"inputs"`
	TargetLanguageCode  string   `json:"target_language_code"`
	Speaker             string   `json:"speaker"`
	Pitch               int      `json:"pitch"`
	Pace                float64  `json:"pace"`
	Loudness            float64  `json:"loudness"`
	SpeechSampleRate    int      `json:"speech_sample_rate"`
	EnablePreprocessing bool     `json:"enable_preprocessing"`
}

type ttsResponse struct {
	Audios []string `json:"audios"`
}

// TTS calls Sarvam text-to-speech and returns the first audio result.
// Errors: "tts-key-missing" when no usable key is configured.
func TTS(ctx context.Context, cfg *config.Config, text string) (string, error) {
	key := cfg.SarvamKey
	if key == "" || key == "your_sarvam_api_key_here" {
		return "", errors.New("tts-key-missing")
	}

	ttsText := strings.TrimSpace(text)
	if len(ttsText) > 450 {
		ttsText = ttsText[:450] + "..."
	}

	body, _ := json.Marshal(ttsRequest{
		Inputs:              []string{ttsText},
		TargetLanguageCode:  "hi-IN",
		Speaker:             "priya",
		Pitch:               0,
		Pace:                1.0,
		Loudness:            1.5,
		SpeechSampleRate:    22050,
		EnablePreprocessing: true,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.sarvam.ai/text-to-speech", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-subscription-key", key)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("TTS API error: status %d", resp.StatusCode)
	}
	var out ttsResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if len(out.Audios) == 0 {
		return "", errors.New("invalid response from TTS service")
	}
	return out.Audios[0], nil
}
