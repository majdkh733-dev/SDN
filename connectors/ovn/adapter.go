// Package ovn is the OVN/OVS northbound adapter. It shells out to ovn-nbctl
// and does not invent a forwarding implementation. This deployment has not
// verified the adapter against a physical lab.
package ovn

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return string(out), nil
}

type LogicalSwitch struct {
	Name string
	UUID string
}

type Adapter struct {
	Runner Runner
	NB     string
}

func (a Adapter) Available(ctx context.Context) error {
	if a.Runner == nil {
		return fmt.Errorf("ovn runner is not configured")
	}
	if _, err := a.Runner.Run(ctx, "ovn-nbctl", "--version"); err != nil {
		return fmt.Errorf("ovn-nbctl is not available: %w", err)
	}
	return nil
}

func (a Adapter) ListLogicalSwitches(ctx context.Context) ([]LogicalSwitch, error) {
	args := []string{"ls-list"}
	if a.NB != "" {
		args = append([]string{"--db=" + a.NB}, args...)
	}
	out, err := a.Runner.Run(ctx, "ovn-nbctl", args...)
	if err != nil {
		return nil, err
	}
	var switches []LogicalSwitch
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switches = append(switches, LogicalSwitch{UUID: fields[0], Name: strings.Trim(fields[len(fields)-1], "()")})
	}
	return switches, nil
}
