// Package adapter defines the connector boundary. The browser never calls these
// types. The API and the site agent do.
package adapter

import "context"

type DeviceBlueprint struct {
	Name       string   `json:"name"`
	Model      string   `json:"model"`
	OSVersion  string   `json:"os_version"`
	Role       string   `json:"role"`
	Interfaces []string `json:"interfaces"`
}

type Discovery struct {
	DataOrigin   string            `json:"data_origin"`
	SupportNote  string            `json:"support_note"`
	Devices      []DeviceBlueprint `json:"devices"`
	Capabilities []string          `json:"capabilities"`
}

type ApplyRequest struct {
	DeviceID        string
	PolicyVersionID string
	Spec            any
	FailWrite       bool
	Uncertain       bool
	Committed       bool
}

type ApplyResult struct {
	Status string
	Detail string
}

type Adapter interface {
	Kind() string
	Discover(ctx context.Context, siteName string) (Discovery, error)
	Apply(ctx context.Context, req ApplyRequest) (ApplyResult, error)
	Remove(ctx context.Context, deviceID, policyVersionID string, fail bool) (ApplyResult, error)
}

func SimulatorDiscovery(siteName string) Discovery {
	prefix := slug(siteName)
	return Discovery{
		DataOrigin:  "simulator",
		SupportNote: "Deterministic simulator. These devices are not forwarding traffic.",
		Capabilities: []string{
			"inventory.v1",
			"acl.v1",
			"backup.v1",
		},
		Devices: []DeviceBlueprint{
			{Name: prefix + "-sw-1", Model: "SDN-SIM-LEAF", OSVersion: "sim-1.0.0", Role: "leaf", Interfaces: []string{"eth1", "eth2"}},
			{Name: prefix + "-sw-2", Model: "SDN-SIM-LEAF", OSVersion: "sim-1.0.0", Role: "leaf", Interfaces: []string{"eth1", "eth2"}},
		},
	}
}

func slug(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r+'a'-'A')
		default:
			if len(out) > 0 && out[len(out)-1] != '-' {
				out = append(out, '-')
			}
		}
	}
	if len(out) == 0 {
		return "site"
	}
	return string(out)
}
