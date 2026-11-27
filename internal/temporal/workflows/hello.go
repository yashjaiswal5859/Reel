package workflows

import (
	"time"

	"go.temporal.io/sdk/workflow"
	"reel/internal/temporal/activities"
)

// HelloWorkflow is a Sprint 0 smoke-test workflow to verify Temporal wiring.
func HelloWorkflow(ctx workflow.Context, name string) (string, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result string
	err := workflow.ExecuteActivity(ctx, activities.HelloActivity, name).Get(ctx, &result)
	if err != nil {
		return "", err
	}
	return result, nil
}
