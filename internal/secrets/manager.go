package secrets

import (
	"context"
	"fmt"
	"sync"
)

// Manager stores device credentials outside PostgreSQL. The database keeps
// only a reference such as vault://org/connector/management.
type Manager interface {
	Put(ctx context.Context, ref, value string) error
	Resolve(ctx context.Context, ref string) (string, error)
}

type Memory struct {
	mu   sync.Mutex
	data map[string]string
}

func NewMemory() *Memory { return &Memory{data: map[string]string{}} }

func (m *Memory) Put(_ context.Context, ref, value string) error {
	if ref == "" || value == "" {
		return fmt.Errorf("secret reference and value are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[ref] = value
	return nil
}

func (m *Memory) Resolve(_ context.Context, ref string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.data[ref]
	if !ok {
		return "", fmt.Errorf("secret %s is not available to this process", ref)
	}
	return value, nil
}

func Reference(orgID, connectorID string) string {
	return "vault://" + orgID + "/" + connectorID + "/management"
}
