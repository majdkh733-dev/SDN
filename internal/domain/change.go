package domain

var changeTransitions = map[string]map[string]bool{
	"validated":    {"approved": true, "rejected": true, "cancelled": true},
	"approved":     {"queued": true, "cancelled": true},
	"queued":       {"applying": true, "cancelled": true},
	"applying":     {"verifying": true, "partial": true, "failed": true, "unknown": true, "cancelled": true},
	"verifying":    {"succeeded": true, "failed": true, "unknown": true},
	"partial":      {"compensating": true, "rollback_failed": true},
	"compensating": {"compensated": true, "rollback_failed": true},
}

func CanTransition(from, to string) bool {
	return changeTransitions[from][to]
}
