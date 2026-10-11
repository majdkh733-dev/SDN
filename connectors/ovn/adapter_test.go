package ovn

import (
	"context"
	"strings"
	"testing"
)

type fakeRunner struct {
	out string
	err error
}

func (f fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	if name != "ovn-nbctl" {
		return "", f.err
	}
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "--version") && f.err != nil && f.out == "" {
		return "", f.err
	}
	if strings.Contains(joined, "ls-list") {
		return f.out, f.err
	}
	return "ovn-nbctl 24.03", nil
}

func TestListLogicalSwitches(t *testing.T) {
	adapter := Adapter{Runner: fakeRunner{out: "uuid-1 (ls-finance)\nuuid-2 (ls-guest)\n"}}
	got, err := adapter.ListLogicalSwitches(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "ls-finance" || got[1].UUID != "uuid-2" {
		t.Fatalf("unexpected switches: %+v", got)
	}
}

func TestUnavailable(t *testing.T) {
	adapter := Adapter{Runner: fakeRunner{err: context.Canceled}}
	if err := adapter.Available(context.Background()); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unavailable, got %v", err)
	}
}
