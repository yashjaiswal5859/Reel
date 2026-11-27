package storage

import (
	"context"
	"fmt"
	"strings"

	"reel/internal/config"
)

// New creates a storage Strategy based on STORAGE_PROVIDER ("local" or "minio"/"s3").
func New(ctx context.Context, cfg config.StorageConfig) (Strategy, error) {
	switch strings.ToLower(cfg.Provider) {
	case "local", "":
		return NewLocalStorage(cfg.LocalDir)
	case "minio", "s3":
		return NewS3Storage(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported storage provider: %q", cfg.Provider)
	}
}
