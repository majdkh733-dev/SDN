package domain

type Role string

const (
	RoleViewer   Role = "viewer"
	RoleOperator Role = "operator"
	RoleEngineer Role = "engineer"
	RoleApprover Role = "approver"
	RoleAdmin    Role = "administrator"
	RoleRecovery Role = "recovery_operator"
)

func CanRead(role Role) bool {
	switch role {
	case RoleViewer, RoleOperator, RoleEngineer, RoleApprover, RoleAdmin, RoleRecovery:
		return true
	default:
		return false
	}
}

func CanOperate(role Role) bool {
	return role == RoleOperator || role == RoleEngineer || role == RoleAdmin
}

func CanAuthor(role Role) bool {
	return role == RoleEngineer || role == RoleAdmin
}

func CanApprove(role Role) bool {
	return role == RoleApprover || role == RoleAdmin
}

func CanAdminister(role Role) bool {
	return role == RoleAdmin
}

func CanRestore(role Role) bool {
	return role == RoleRecovery || role == RoleAdmin
}
