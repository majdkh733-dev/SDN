package domain

import "testing"

func TestRoles(t *testing.T) {
	if CanAuthor(RoleViewer) || CanApprove(RoleEngineer) || !CanApprove(RoleApprover) {
		t.Fatal("role matrix mismatch")
	}
	if !CanRestore(RoleRecovery) || CanRestore(RoleEngineer) || !CanAdminister(RoleAdmin) {
		t.Fatal("restore and admin matrix mismatch")
	}
	if CanRead(Role("stranger")) {
		t.Fatal("unknown role must not read")
	}
}
