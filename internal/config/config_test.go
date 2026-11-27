package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"reel/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := config.Load("")
	require.NoError(t, err)

	assert.Equal(t, "development", cfg.App.Env)
	assert.Equal(t, 8080, cfg.App.Port)
	assert.Equal(t, "localhost:7233", cfg.Temporal.Host)
	assert.Equal(t, "local", cfg.Storage.Provider)
	assert.Equal(t, "groq", cfg.AI.ASRProvider)
	assert.Equal(t, "groq", cfg.AI.LLMProvider)
	assert.Equal(t, 600, cfg.Limits.MaxVideoDurationSec)
}

func TestLoad_FromEnvFileAndOverride(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	err := os.WriteFile(envPath, []byte("APP_PORT=9090\nASR_PROVIDER=openai\nGROQ_API_KEY=test-groq-key\n"), 0600)
	require.NoError(t, err)

	t.Setenv("LLM_PROVIDER", "anthropic")

	cfg, err := config.Load(envPath)
	require.NoError(t, err)

	assert.Equal(t, 9090, cfg.App.Port)
	assert.Equal(t, "openai", cfg.AI.ASRProvider)
	assert.Equal(t, "test-groq-key", cfg.AI.GroqAPIKey)
	assert.Equal(t, "anthropic", cfg.AI.LLMProvider)
}

func TestLoad_MissingFileReturnsError(t *testing.T) {
	_, err := config.Load("/nonexistent/.env")
	require.Error(t, err)
}
