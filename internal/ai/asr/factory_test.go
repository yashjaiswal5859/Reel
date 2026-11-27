package asr_test

import (
	"context"
	"errors"
	"testing"

	"reel/internal/ai/asr"
	"reel/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubASR struct {
	name string
	fail bool
}

func (s *stubASR) Name() string { return s.name }
func (s *stubASR) Transcribe(_ context.Context, _ asr.TranscribeRequest) (*asr.TranscribeResponse, error) {
	if s.fail {
		return nil, errors.New("boom")
	}
	return &asr.TranscribeResponse{Provider: s.name, Text: "ok"}, nil
}

func TestASRFactory_AndFallback(t *testing.T) {
	asr.Register("stub-primary", func(_ config.AIConfig) (asr.Strategy, error) {
		return &stubASR{name: "stub-primary", fail: true}, nil
	})
	asr.Register("stub-backup", func(_ config.AIConfig) (asr.Strategy, error) {
		return &stubASR{name: "stub-backup", fail: false}, nil
	})

	p, err := asr.NewWithFallback(config.AIConfig{
		ASRProvider:         "stub-primary",
		ASRFallbackProvider: "stub-backup",
	})
	require.NoError(t, err)
	assert.Equal(t, "stub-primary+stub-backup", p.Name())

	resp, err := p.Transcribe(context.Background(), asr.TranscribeRequest{})
	require.NoError(t, err)
	assert.Equal(t, "stub-backup", resp.Provider)
}

func TestASRFactory_UnknownProvider(t *testing.T) {
	_, err := asr.New(config.AIConfig{ASRProvider: "unknown-asr"})
	require.Error(t, err)
}
