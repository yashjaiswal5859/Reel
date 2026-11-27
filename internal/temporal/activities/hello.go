package activities

import (
	"context"
	"fmt"
)

// HelloActivity is a Sprint 0 smoke-test activity.
func HelloActivity(_ context.Context, name string) (string, error) {
	if name == "" {
		name = "world"
	}
	return fmt.Sprintf("hello, %s", name), nil
}
