package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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

const correctEnglishPrompt = `You are a precise English grammar and spelling correction assistant.

Rules:
- Correct all grammar, spelling, punctuation, and capitalization errors in the given English text
- Preserve the original meaning, tone, and vocabulary choices — do not rephrase or improve style
- If a sentence is already correct, return it unchanged
- Return ONLY the corrected English text
- Do not include quotes, labels, explanations, or annotations
- If the input is empty or not English text, return exactly: [INVALID_INPUT]`

// openRouterRequest is the request body for OpenRouter's chat completions API.
type openRouterRequest struct {
	Model    string               `json:"model"`
	Messages []openRouterMessage  `json:"messages"`
}

type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openRouterResponse mirrors the OpenAI-compatible response shape.
type openRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// OpenRouterClient wraps the OpenRouter API for translation.
type OpenRouterClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewOpenRouterClient creates a new OpenRouter-backed client.
func NewOpenRouterClient(apiKey, model string) (*OpenRouterClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY is required")
	}
	return &OpenRouterClient{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Translate sends the Chinese text to OpenRouter and returns the English translation.
func (c *OpenRouterClient) Translate(ctx context.Context, chinese string) (string, error) {
	payload := openRouterRequest{
		Model: c.model,
		Messages: []openRouterMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf("Chinese: %s", chinese)},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://openrouter.ai/api/v1/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openrouter API call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var result openRouterResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("openrouter error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("openrouter returned empty response")
	}

	text := strings.TrimSpace(result.Choices[0].Message.Content)
	if text == "" {
		return "", fmt.Errorf("openrouter returned empty content")
	}
	if text == "[INVALID_INPUT]" {
		return "", fmt.Errorf("invalid input: not recognized as Chinese text")
	}

	return text, nil
}

// CorrectEnglish sends English text to OpenRouter and returns the grammar-corrected version.
func (c *OpenRouterClient) CorrectEnglish(ctx context.Context, english string) (string, error) {
	payload := openRouterRequest{
		Model: c.model,
		Messages: []openRouterMessage{
			{Role: "system", Content: correctEnglishPrompt},
			{Role: "user", Content: english},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://openrouter.ai/api/v1/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openrouter API call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var result openRouterResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("openrouter error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("openrouter returned empty response")
	}

	text := strings.TrimSpace(result.Choices[0].Message.Content)
	if text == "" {
		return "", fmt.Errorf("openrouter returned empty content")
	}
	if text == "[INVALID_INPUT]" {
		return "", fmt.Errorf("invalid input: not recognized as English text")
	}

	return text, nil
}
