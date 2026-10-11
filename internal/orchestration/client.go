package orchestration

import (
	"context"
	"errors"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

type Client struct {
	raw client.Client
}

func Dial(address, namespace string) (*Client, error) {
	if namespace == "" {
		namespace = "default"
	}
	raw, err := client.Dial(client.Options{HostPort: address, Namespace: namespace})
	if err != nil {
		return nil, err
	}
	return &Client{raw: raw}, nil
}

func (c *Client) Close() {
	if c != nil && c.raw != nil {
		c.raw.Close()
	}
}

func (c *Client) StartChange(ctx context.Context, orgID, changeID string) error {
	_, err := c.raw.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                    "sdn-change-" + changeID,
		TaskQueue:             TaskQueue,
		WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
	}, ExecuteChange, ChangeInput{OrganizationID: orgID, ChangeID: changeID})
	var already *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &already) {
		return nil
	}
	return err
}

// RunWorker blocks until the process is interrupted. It polls TEMPORAL_ADDRESS.
// It does not claim PostgreSQL rows; the API starts one workflow per change.
func RunWorker(ctx context.Context, address, namespace string, run func(context.Context, string, string) error) error {
	c, err := Dial(address, namespace)
	if err != nil {
		return err
	}
	defer c.Close()
	w := worker.New(c.raw, TaskQueue, worker.Options{})
	w.RegisterWorkflow(ExecuteChange)
	w.RegisterActivity(&Activities{Run: run})
	errCh := make(chan error, 1)
	go func() {
		errCh <- w.Run(worker.InterruptCh())
	}()
	select {
	case <-ctx.Done():
		w.Stop()
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
