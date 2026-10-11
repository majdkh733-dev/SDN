package orchestration

import (
	"context"
	"errors"
	"testing"

	"go.temporal.io/sdk/testsuite"
)

func TestExecuteChangeUsesTheSuppliedIdentifiers(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var got ChangeInput
	env.RegisterActivity(&Activities{Run: func(_ context.Context, orgID, changeID string) error {
		got = ChangeInput{OrganizationID: orgID, ChangeID: changeID}
		return nil
	}})
	env.ExecuteWorkflow(ExecuteChange, ChangeInput{OrganizationID: "org-1", ChangeID: "change-1"})
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	if got.OrganizationID != "org-1" || got.ChangeID != "change-1" {
		t.Fatalf("activity received %#v", got)
	}
}

func TestExecuteChangeSurfacesActivityFailure(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivity(&Activities{Run: func(context.Context, string, string) error {
		return errors.New("device outcome is unknown")
	}})
	env.ExecuteWorkflow(ExecuteChange, ChangeInput{OrganizationID: "org-1", ChangeID: "change-1"})
	if err := env.GetWorkflowError(); err == nil {
		t.Fatal("expected activity error")
	}
}

func TestExecuteChangeRejectsAnEmptyPlan(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	called := false
	env.RegisterActivity(&Activities{Run: func(context.Context, string, string) error {
		called = true
		return nil
	}})
	env.ExecuteWorkflow(ExecuteChange, ChangeInput{})
	if err := env.GetWorkflowError(); err == nil {
		t.Fatal("expected invalid input")
	}
	if called {
		t.Fatal("activity ran without identifiers")
	}
}
