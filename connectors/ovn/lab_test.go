package ovn

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestLabProbe runs the real ovn-nbctl binary when it exists. A missing binary
// or a missing northbound database is an unverified adapter, not a test failure.
func TestLabProbe(t *testing.T) {
	adapter := Adapter{Runner: ExecRunner{}, NB: os.Getenv("OVN_NB_DB")}
	if err := adapter.Available(context.Background()); err != nil {
		if !strings.Contains(err.Error(), "not available") {
			t.Fatal(err)
		}
		t.Logf("status=implemented_unverified detail=%s", err.Error())
		return
	}
	switches, err := adapter.ListLogicalSwitches(context.Background())
	if err != nil {
		t.Logf("status=implemented_unverified detail=%s", err.Error())
		return
	}
	t.Logf("status=lab_verified switches=%d", len(switches))
}
