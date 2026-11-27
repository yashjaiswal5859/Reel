package imagegen

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const maxImageDownloadBytes = 25 * 1024 * 1024 // 25MB cap per generated image

var sharedHTTPClient = &http.Client{
	Timeout: 3 * time.Minute,
	Transport: &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 24,
		IdleConnTimeout:     90 * time.Second,
	},
}

func downloadImage(ctx context.Context, hc *http.Client, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create image download request: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("image download status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageDownloadBytes))
	if err != nil {
		return nil, "", fmt.Errorf("read image bytes: %w", err)
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/png"
	}
	return data, ct, nil
}
