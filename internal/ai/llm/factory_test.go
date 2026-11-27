package llm_test

import (
	"context"
	"errors"
	"testing"

	"reel/internal/ai/llm"
	"reel/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubLLM struct {
	name string
	fail bool
}

func (s *stubLLM) Name() string { return s.name }
func (s *stubLLM) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, error) {
	if s.fail {
		return nil, errors.New("rate limited")
	}
	return &llm.GenerateResponse{Provider: s.name, Content: "hello"}, nil
}

func TestLLMFactory_AndFallback(t *testing.T) {
	llm.Register("stub-groq", func(_ config.AIConfig) (llm.Strategy, error) {
		return &stubLLM{name: "stub-groq", fail: true}, nil
	})
	llm.Register("stub-openai", func(_ config.AIConfig) (llm.Strategy, error) {
		return &stubLLM{name: "stub-openai", fail: false}, nil
	})

	p, err := llm.NewWithFallback(config.AIConfig{
		LLMProvider:         "stub-groq",
		LLMFallbackProvider: "stub-openai",
	})
	require.NoError(t, err)

	resp, err := p.Generate(context.Background(), llm.GenerateRequest{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "stub-openai", resp.Provider)
	assert.Equal(t, "hello", resp.Content)
}

func TestLLMFactory_UnknownProvider(t *testing.T) {
	_, err := llm.New(config.AIConfig{LLMProvider: "unknown-llm"})
	require.Error(t, err)
}
