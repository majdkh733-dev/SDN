package domain

import (
	"strings"
	"testing"
	"time"
)

func sampleSpec() PolicySpec {
	return PolicySpec{
		Name:        "finance-apps",
		TenantID:    "ten-1",
		Source:      Selector{Type: "segment", ID: "seg-a"},
		Destination: Selector{Type: "segment", ID: "seg-b"},
		Protocol:    "tcp",
		Ports:       "443",
		Action:      "allow",
		SiteIDs:     []string{"site-b", "site-a"},
	}
}

func TestPolicyShape(t *testing.T) {
	spec := NormalizeSpec(sampleSpec())
	if len(spec.ValidateShape()) != 0 {
		t.Fatalf("expected valid spec, got %v", spec.ValidateShape())
	}
	if spec.SiteIDs[0] != "site-a" {
		t.Fatal("site ids must be sorted for a stable hash")
	}
	bad := spec
	bad.Action = "log"
	if len(bad.ValidateShape()) == 0 {
		t.Fatal("expected action error")
	}
}

func TestPlanHashStable(t *testing.T) {
	spec := NormalizeSpec(sampleSpec())
	impact := Impact{
		PolicyVersionID: "ver-1",
		Items: []ImpactItem{
			{DeviceID: "b", DeviceName: "leaf-b", Classification: "calculated", Action: "add_acl", Detail: "allow", Blocking: false},
			{DeviceID: "a", DeviceName: "leaf-a", Classification: "calculated", Action: "add_acl", Detail: "allow", Blocking: false},
		},
		Calculated: 2,
	}
	impact.Items = sortItems(impact.Items)
	h1, err := CanonicalHash(struct {
		Spec   PolicySpec `json:"spec"`
		Impact Impact     `json:"impact"`
	}{spec, impact})
	if err != nil {
		t.Fatal(err)
	}
	h2, err := CanonicalHash(struct {
		Spec   PolicySpec `json:"spec"`
		Impact Impact     `json:"impact"`
	}{spec, impact})
	if err != nil || h1 != h2 || len(h1) != 64 {
		t.Fatalf("hash not stable: %s %s %v", h1, h2, err)
	}
}

func sortItems(items []ImpactItem) []ImpactItem {
	out := append([]ImpactItem(nil), items...)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].DeviceID < out[i].DeviceID {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func TestPreviewToken(t *testing.T) {
	exp := time.Now().Add(time.Minute)
	token := SignPreview("secret", "ver", "abc", exp)
	version, hash, err := VerifyPreview("secret", token, time.Now())
	if err != nil || version != "ver" || hash != "abc" {
		t.Fatalf("verify failed: %s %s %v", version, hash, err)
	}
	if _, _, err := VerifyPreview("other", token, time.Now()); err == nil {
		t.Fatal("expected bad signature")
	}
	expired := SignPreview("secret", "ver", "abc", time.Now().Add(-time.Second))
	if _, _, err := VerifyPreview("secret", expired, time.Now()); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatal("expected expiry")
	}
}
