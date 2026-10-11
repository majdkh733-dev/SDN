package domain

import "testing"

func TestChangeTransitions(t *testing.T) {
	if !CanTransition("validated", "approved") || CanTransition("validated", "succeeded") {
		t.Fatal("approval gate mismatch")
	}
	if !CanTransition("partial", "compensating") || CanTransition("succeeded", "applying") {
		t.Fatal("compensation gate mismatch")
	}
}
