package fallback_test

import (
	"context"
	"errors"
	"testing"

	"reel/internal/ai/fallback"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecute_PrimarySucceeds(t *testing.T) {
	got, err := fallback.Execute(
		context.Background(),
		"groq",
		func(_ context.Context, s string) (string, error) { return "ok:" + s, nil },
		"openai",
		func(_ context.Context, s string) (string, error) { return "fb:" + s, nil },
		"hi",
	)
	require.NoError(t, err)
	assert.Equal(t, "ok:hi", got)
}

func TestExecute_FallsBackOnPrimaryError(t *testing.T) {
	got, err := fallback.Execute(
		context.Background(),
		"groq",
		func(_ context.Context, _ string) (string, error) { return "", errors.New("rate limit") },
		"openai",
		func(_ context.Context, s string) (string, error) { return "fb:" + s, nil },
		"hi",
	)
	require.NoError(t, err)
	assert.Equal(t, "fb:hi", got)
}

func TestExecute_BothFailReturnsCombinedError(t *testing.T) {
	_, err := fallback.Execute(
		context.Background(),
		"groq",
		func(_ context.Context, _ string) (string, error) { return "", errors.New("p-err") },
		"openai",
		func(_ context.Context, _ string) (string, error) { return "", errors.New("fb-err") },
		"hi",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "groq")
	assert.Contains(t, err.Error(), "openai")
}
