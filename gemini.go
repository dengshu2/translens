package main

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

const systemPrompt = `You are a professional Chinese-to-English translation assistant specializing in natural, everyday American English.

Rules:
- Translate the given Chinese text into casual, colloquial American English
- Use expressions that a native speaker would actually say in daily conversation
- Return ONLY the translated English text
- Do not include quotes, labels, prefixes, suffixes, or punctuation wrappers
- Do not add explanations, alternatives, or annotations
- Maintain the original tone and intent of the Chinese text
- If the input is empty or not Chinese, return exactly: [INVALID_INPUT]`

// GeminiClient wraps the Google GenAI client for translation.
type GeminiClient struct {
	client *genai.Client
	model  string
}

// NewGeminiClient creates a new GeminiClient with the given API key and model.
func NewGeminiClient(apiKey, model string) (*GeminiClient, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("create genai client: %w", err)
	}

	return &GeminiClient{
		client: client,
		model:  model,
	}, nil
}

// Translate sends the Chinese text to Gemini and returns the English translation.
func (gc *GeminiClient) Translate(ctx context.Context, chinese string) (string, error) {
	temperature := float32(0.4)
	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemPrompt, "user"),
		Temperature:       &temperature,
	}

	result, err := gc.client.Models.GenerateContent(
		ctx,
		gc.model,
		genai.Text(fmt.Sprintf("Chinese: %s", chinese)),
		config,
	)
	if err != nil {
		return "", fmt.Errorf("gemini API call failed: %w", err)
	}

	text := strings.TrimSpace(result.Text())
	if text == "" {
		return "", fmt.Errorf("gemini returned empty response")
	}
	if text == "[INVALID_INPUT]" {
		return "", fmt.Errorf("invalid input: not recognized as Chinese text")
	}

	return text, nil
}
