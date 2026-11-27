package workflows_test

import (
	"testing"

	"reel/internal/temporal/activities"
	"reel/internal/temporal/workflows"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func TestHelloWorkflow(t *testing.T) {
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterActivity(activities.HelloActivity)

	env.ExecuteWorkflow(workflows.HelloWorkflow, "reel")
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result string
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, "hello, reel", result)
}
