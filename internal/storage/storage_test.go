package storage_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"reel/internal/config"
	"reel/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStorage_CRUDAndCleanup(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	st, err := storage.New(ctx, config.StorageConfig{
		Provider: "local",
		LocalDir: dir,
	})
	require.NoError(t, err)

	key := "jobs/job-1/input.mp4"
	payload := []byte("hello-video-bytes")

	require.NoError(t, st.Put(ctx, key, bytes.NewReader(payload), int64(len(payload)), "video/mp4"))

	exists, err := st.Exists(ctx, key)
	require.NoError(t, err)
	assert.True(t, exists)

	rc, err := st.Get(ctx, key)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	assert.Equal(t, payload, got)

	// Make file appear old to test TTL cleanup
	oldTime := time.Now().Add(-48 * time.Hour)
	fullPath := filepath.Join(dir, "jobs", "job-1", "input.mp4")
	require.NoError(t, os.Chtimes(fullPath, oldTime, oldTime))

	removed, err := st.CleanupExpired(ctx, 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)

	exists, err = st.Exists(ctx, key)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestLocalStorage_PathTraversalRejected(t *testing.T) {
	ctx := context.Background()
	st, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)

	err = st.Put(ctx, "../escape.txt", bytes.NewReader([]byte("x")), 1, "text/plain")
	require.Error(t, err)
}

func TestFactory_UnsupportedProvider(t *testing.T) {
	_, err := storage.New(context.Background(), config.StorageConfig{Provider: "gcs"})
	require.Error(t, err)
}
