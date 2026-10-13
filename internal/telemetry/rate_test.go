package telemetry

import (
	"testing"
	"time"
)

func TestUtilization(t *testing.T) {
	got, ok := Utilization(0, 125_000, time.Second, 1_000_000)
	if !ok || got < 99 || got > 101 {
		t.Fatalf("utilization %v %v", got, ok)
	}
	if _, ok := Utilization(10, 9, time.Second, 1_000_000); ok {
		t.Fatal("counter reset must not produce a rate")
	}
	if _, ok := Utilization(0, 10, 0, 1_000_000); ok {
		t.Fatal("missing elapsed time")
	}
	if _, ok := Utilization(0, 10, time.Second, 0); ok {
		t.Fatal("unknown speed")
	}
}
