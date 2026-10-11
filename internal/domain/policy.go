package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Selector struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type PolicySpec struct {
	Name        string   `json:"name"`
	TenantID    string   `json:"tenant_id"`
	Source      Selector `json:"source"`
	Destination Selector `json:"destination"`
	Protocol    string   `json:"protocol"`
	Ports       string   `json:"ports"`
	Action      string   `json:"action"`
	SiteIDs     []string `json:"site_ids"`
}

type ImpactItem struct {
	DeviceID       string `json:"device_id"`
	DeviceName     string `json:"device_name"`
	Classification string `json:"classification"`
	Action         string `json:"action"`
	Detail         string `json:"detail"`
	Blocking       bool   `json:"blocking"`
}

type Impact struct {
	PolicyVersionID string       `json:"policy_version_id"`
	Items           []ImpactItem `json:"items"`
	Blocked         bool         `json:"blocked"`
	BlockReason     string       `json:"block_reason,omitempty"`
	Calculated      int          `json:"calculated"`
	Inferred        int          `json:"inferred"`
	Verified        int          `json:"verified"`
	Unknown         int          `json:"unknown"`
}

func (s PolicySpec) ValidateShape() []string {
	var errs []string
	if strings.TrimSpace(s.Name) == "" {
		errs = append(errs, "name is required")
	}
	if s.TenantID == "" {
		errs = append(errs, "tenant_id is required")
	}
	if s.Source.Type != "segment" || s.Source.ID == "" {
		errs = append(errs, "source must be a segment selector")
	}
	if s.Destination.Type != "segment" || s.Destination.ID == "" {
		errs = append(errs, "destination must be a segment selector")
	}
	if s.Source.ID != "" && s.Source.ID == s.Destination.ID {
		errs = append(errs, "source and destination must differ")
	}
	switch s.Protocol {
	case "tcp", "udp", "icmp", "any":
	default:
		errs = append(errs, "protocol must be tcp, udp, icmp, or any")
	}
	switch s.Action {
	case "allow", "deny":
	default:
		errs = append(errs, "action must be allow or deny")
	}
	if s.Protocol == "tcp" || s.Protocol == "udp" {
		if s.Ports == "" || !validPorts(s.Ports) {
			errs = append(errs, "ports must be a single port or a range such as 443 or 1000-2000")
		}
	}
	if len(s.SiteIDs) == 0 {
		errs = append(errs, "at least one site is required")
	}
	return errs
}

func validPorts(raw string) bool {
	if strings.Contains(raw, "-") {
		parts := strings.Split(raw, "-")
		if len(parts) != 2 {
			return false
		}
		a, aok := atoiPort(parts[0])
		b, bok := atoiPort(parts[1])
		return aok && bok && a < b
	}
	_, ok := atoiPort(raw)
	return ok
}

func atoiPort(raw string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 1 || n > 65535 {
		return 0, false
	}
	return n, true
}

func CanonicalHash(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func SignPreview(secret, versionID, hash string, exp time.Time) string {
	payload := versionID + "|" + hash + "|" + strconv.FormatInt(exp.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func VerifyPreview(secret, token string, now time.Time) (versionID, hash string, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("malformed preview token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("malformed preview token")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("malformed preview token")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	if subtle.ConstantTimeCompare(mac.Sum(nil), sig) != 1 {
		return "", "", fmt.Errorf("preview token signature mismatch")
	}
	fields := strings.Split(string(payload), "|")
	if len(fields) != 3 {
		return "", "", fmt.Errorf("malformed preview token")
	}
	expUnix, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return "", "", fmt.Errorf("malformed preview token")
	}
	if now.After(time.Unix(expUnix, 0)) {
		return "", "", fmt.Errorf("preview token expired")
	}
	return fields[0], fields[1], nil
}

func SortIDs(ids []string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}

func NormalizeSpec(spec PolicySpec) PolicySpec {
	spec.Name = strings.TrimSpace(spec.Name)
	spec.Protocol = strings.ToLower(strings.TrimSpace(spec.Protocol))
	spec.Action = strings.ToLower(strings.TrimSpace(spec.Action))
	spec.Ports = strings.TrimSpace(spec.Ports)
	spec.SiteIDs = SortIDs(spec.SiteIDs)
	return spec
}
