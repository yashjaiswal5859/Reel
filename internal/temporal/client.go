package temporalclient

import (
	"fmt"

	"go.temporal.io/sdk/client"
	"reel/internal/config"
)

// New creates a Temporal SDK client from TemporalConfig.
func New(cfg config.TemporalConfig) (client.Client, error) {
	c, err := client.Dial(client.Options{
		HostPort:  cfg.Host,
		Namespace: cfg.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("dial temporal %s: %w", cfg.Host, err)
	}
	return c, nil
}
