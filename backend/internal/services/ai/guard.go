// Package ai — guard.go holds the Phase-1 AI policy (finding S3): a
// general-assistant system instruction, the pre-model emergency-intent
// branch, and a context-aware chat call that reuses the Groq->OpenRouter
// fallback chain. No request logic, no quotas — those live in the chatbot
// handler (internal/hands).
package ai

import (
	"context"
	"errors"
	"strings"

	"healthtrack/internal/config"
)

// AssistantInstruction replaces the legacy "medical assistant" system prompt
// (Phase 0 confirmed it as a compliance conflict). It is a general
// wellness-information assistant: it never diagnoses, never prescribes,
// never ranks diseases, and always carries the non-clinical disclaimer.
const AssistantInstruction = "You are the HealthTrack+ general assistant. " +
	"You provide general health, nutrition, and fitness information and help users understand their own data and use the app. " +
	"You are NOT a clinician: never diagnose, never prescribe or adjust medication, never rank or suggest specific diseases, and never give dosing advice. " +
	"When a user describes symptoms, say you cannot diagnose and direct them to a qualified clinician. " +
	"Always end clinical-topic answers with: \"This is general information, not medical advice — consult a qualified healthcare professional about your situation.\""

// emergencyMarkers are the intent cues checked BEFORE any model call
// (finding S3). Deliberate substring match on lower-cased input; a hit
// returns the fixed emergency reply and skips the provider entirely.
var emergencyMarkers = []string{
	"suicide", "suicid", "kill myself", "end my life", "self harm", "self-harm",
	"chest pain", "can't breathe", "cannot breathe", "trouble breathing", "shortness of breath",
	"heart attack", "stroke", "unconscious", "severe bleeding", "overdose",
}

// emergencyReply is the fixed, non-diagnostic response. It names the
// national emergency numbers (112 all-india / 108 ambulance) and always
// says "contact emergency services / qualified clinicians" — it never
// diagnoses, never ranks causes, never prescribes.
const emergencyReply = "If you or someone else is in immediate danger, please call your local emergency number now: " +
	"112 (all services) or 108 (ambulance) in India. " +
	"Contact emergency services or a qualified clinician as soon as possible; do not wait for an online reply. " +
	"I am a general information assistant and cannot assess or diagnose your condition. " +
	"If this is a mental-health crisis, consider contacting a crisis helpline (e.g. iCall: +91 91-98189-98189, Vandrevala: +91 9999-666-999) or a trusted person nearby, in addition to emergency services."

// IsEmergencyIntent reports whether the message contains emergency intent.
// Call this BEFORE invoking any model so the emergency path costs no
// provider call and cannot be bypassed by the model's own judgment.
func IsEmergencyIntent(message string) bool {
	low := strings.ToLower(message)
	for _, m := range emergencyMarkers {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}

// EmergencyReply is the fixed emergency response (see emergencyReply).
func EmergencyReply() string { return emergencyReply }

// ChatWithInstruction runs the Groq->OpenRouter fallback chain with a
// caller-chosen system instruction. It is the context-aware entry point the
// chatbot uses for the general-assistant instruction; the plain Chat()
// keeps its legacy instruction for backward compatibility.
func ChatWithInstruction(ctx context.Context, cfg *config.Config, message, instruction string) (string, error) {
	msgs := []chatMessage{
		{Role: "system", Content: instruction},
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
	return postChat(ctx, "https://openrouter.ai/api/v1/chat/completions", "google/gemini-2.0-flash-001", key, msgs)
}
