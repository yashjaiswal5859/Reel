package storage

import (
	"context"
	"io"
	"time"
)

// Strategy defines the storage behavior for temporary video/audio/image assets.
type Strategy interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	URL(ctx context.Context, key string, expiry time.Duration) (string, error)
	CleanupExpired(ctx context.Context, maxAge time.Duration) (int, error)
}
