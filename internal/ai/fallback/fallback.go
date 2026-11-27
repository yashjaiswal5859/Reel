package fallback

import (
	"context"
	"fmt"
)

// Execute runs primaryFn and, if it fails and fallbackFn is non-nil, falls back to fallbackFn.
func Execute[Req any, Resp any](
	ctx context.Context,
	primaryName string,
	primaryFn func(context.Context, Req) (Resp, error),
	fallbackName string,
	fallbackFn func(context.Context, Req) (Resp, error),
	req Req,
) (Resp, error) {
	resp, err := primaryFn(ctx, req)
	if err == nil {
		return resp, nil
	}
	if fallbackFn == nil {
		var zero Resp
		return zero, fmt.Errorf("%s failed: %w", primaryName, err)
	}

	resp, fbErr := fallbackFn(ctx, req)
	if fbErr != nil {
		var zero Resp
		return zero, fmt.Errorf("%s failed (%v); fallback %s failed: %w", primaryName, err, fallbackName, fbErr)
	}
	return resp, nil
}
