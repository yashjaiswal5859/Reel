package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var copyBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 64*1024)
		return &buf
	},
}

// LocalStorage stores temporary files on the local filesystem.
type LocalStorage struct {
	baseDir string
}

// NewLocalStorage creates a LocalStorage rooted at baseDir.
func NewLocalStorage(baseDir string) (*LocalStorage, error) {
	if baseDir == "" {
		return nil, errors.New("local storage baseDir is required")
	}
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("resolve baseDir: %w", err)
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return nil, fmt.Errorf("create baseDir: %w", err)
	}
	return &LocalStorage{baseDir: abs}, nil
}

func (l *LocalStorage) safePath(key string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(key))
	if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("invalid storage key: %q", key)
	}
	full := filepath.Join(l.baseDir, clean)
	if !strings.HasPrefix(full, l.baseDir+string(os.PathSeparator)) {
		return "", fmt.Errorf("storage key escapes baseDir: %q", key)
	}
	return full, nil
}

func (l *LocalStorage) Put(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	path, err := l.safePath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	bufPtr := copyBufPool.Get().(*[]byte)
	defer copyBufPool.Put(bufPtr)

	if _, err := io.CopyBuffer(f, r, *bufPtr); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func (l *LocalStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	path, err := l.safePath(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	return f, nil
}

func (l *LocalStorage) Delete(_ context.Context, key string) error {
	path, err := l.safePath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove file: %w", err)
	}
	return nil
}

func (l *LocalStorage) Exists(_ context.Context, key string) (bool, error) {
	path, err := l.safePath(key)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func (l *LocalStorage) URL(_ context.Context, key string, _ time.Duration) (string, error) {
	path, err := l.safePath(key)
	if err != nil {
		return "", err
	}
	return path, nil
}

func (l *LocalStorage) CleanupExpired(_ context.Context, maxAge time.Duration) (int, error) {
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	err := filepath.WalkDir(l.baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err == nil {
				removed++
			}
		}
		return nil
	})
	return removed, err
}
