// Package orchestration starts change execution on Temporal when a cluster
// address is configured. The workflow calls the same RunChange path the
// PostgreSQL worker uses. This package does not embed a Temporal server.
package orchestration

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const TaskQueue = "sdn-change-execution"

type ChangeInput struct {
	OrganizationID string `json:"organization_id"`
	ChangeID       string `json:"change_id"`
}

type Activities struct {
	Run func(ctx context.Context, orgID, changeID string) error
}

func (a *Activities) RunChange(ctx context.Context, in ChangeInput) error {
	if a == nil || a.Run == nil {
		return temporal.NewNonRetryableApplicationError("change runner is not configured", "not_configured", nil)
	}
	if in.OrganizationID == "" || in.ChangeID == "" {
		return temporal.NewNonRetryableApplicationError("organization and change are required", "invalid", nil)
	}
	return a.Run(ctx, in.OrganizationID, in.ChangeID)
}

// ExecuteChange is the Temporal workflow. A worker restart resumes it from
// Temporal history. RunChange itself resumes verification and compensation
// from PostgreSQL, using the stored plan.
func ExecuteChange(ctx workflow.Context, in ChangeInput) error {
	if in.OrganizationID == "" || in.ChangeID == "" {
		return temporal.NewNonRetryableApplicationError("organization and change are required", "invalid", nil)
	}
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
	})
	if err := workflow.ExecuteActivity(ctx, (*Activities).RunChange, in).Get(ctx, nil); err != nil {
		return fmt.Errorf("run change: %w", err)
	}
	return nil
}
