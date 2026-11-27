package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"reel/internal/ai/llm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAICompat_GroqGenerate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer groq-secret", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "llama-3.3-70b-versatile",
			"choices": []map[string]any{
				{"message": map[string]any{"content": `{"ok":true}`}},
			},
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		})
	}))
	defer srv.Close()

	client, err := llm.NewOpenAICompatWithEndpoint("groq", srv.URL, "groq-secret", "llama-3.3-70b-versatile", srv.Client())
	require.NoError(t, err)

	resp, err := client.Generate(context.Background(), llm.GenerateRequest{
		Messages: []llm.Message{{Role: "user", Content: "hello"}},
		JSONMode: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "groq", resp.Provider)
	assert.Equal(t, `{"ok":true}`, resp.Content)
	assert.Equal(t, 15, resp.Usage.TotalTokens)
}

func TestGemini_Generate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/gemini-2.0-flash:generateContent")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{"content": map[string]any{"parts": []map[string]any{{"text": "gemini-ok"}}}},
			},
			"usageMetadata": map[string]any{"promptTokenCount": 8, "candidatesTokenCount": 4, "totalTokenCount": 12},
		})
	}))
	defer srv.Close()

	client := llm.NewGeminiWithBaseURL(srv.URL, "gem-key", "gemini-2.0-flash", srv.Client())
	resp, err := client.Generate(context.Background(), llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "be concise"},
			{Role: "user", Content: "hi"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "gemini", resp.Provider)
	assert.Equal(t, "gemini-ok", resp.Content)
	assert.Equal(t, 12, resp.Usage.TotalTokens)
}

func TestAnthropic_Generate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "ant-key", r.Header.Get("x-api-key"))
		assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "claude-sonnet-4-20250514",
			"content": []map[string]any{
				{"type": "text", "text": "claude-ok"},
			},
			"usage": map[string]any{"input_tokens": 12, "output_tokens": 6},
		})
	}))
	defer srv.Close()

	client := llm.NewAnthropicWithEndpoint(srv.URL, "ant-key", "claude-sonnet-4-20250514", srv.Client())
	resp, err := client.Generate(context.Background(), llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "hi"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "anthropic", resp.Provider)
	assert.Equal(t, "claude-ok", resp.Content)
	assert.Equal(t, 18, resp.Usage.TotalTokens)
}
