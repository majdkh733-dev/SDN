package discover

import (
	"fmt"
	"net"
	"strings"
)

const MaxHosts = 256

// Target is one address the connector may probe. Explicit is true when the
// operator named that address, so a silent result is still reported.
type Target struct {
	IP       string
	Explicit bool
}

// Expand validates an allow-list and returns at most MaxHosts addresses.
// IPv4 prefixes shorter than /24 are rejected. Exclusions are applied after expansion.
func Expand(targets, exclusions []string) ([]Target, error) {
	if len(targets) == 0 {
		return nil, fmt.Errorf("at least one target is required")
	}
	excluded := map[string]bool{}
	for _, item := range exclusions {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		ips, err := enumerate(item, true)
		if err != nil {
			return nil, fmt.Errorf("exclusion %s: %w", item, err)
		}
		for _, ip := range ips {
			excluded[ip] = true
		}
	}
	var out []Target
	seen := map[string]bool{}
	for _, item := range targets {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		ips, err := enumerate(item, false)
		if err != nil {
			return nil, fmt.Errorf("target %s: %w", item, err)
		}
		explicit := !strings.Contains(item, "/")
		for _, ip := range ips {
			if excluded[ip] || seen[ip] {
				continue
			}
			seen[ip] = true
			out = append(out, Target{IP: ip, Explicit: explicit})
			if len(out) > MaxHosts {
				return nil, fmt.Errorf("scan exceeds %d addresses", MaxHosts)
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("targets are empty after exclusions")
	}
	return out, nil
}

func enumerate(item string, exclusion bool) ([]string, error) {
	if ip := net.ParseIP(item); ip != nil {
		ip4 := ip.To4()
		if ip4 == nil {
			return nil, fmt.Errorf("only IPv4 targets are supported")
		}
		return []string{ip4.String()}, nil
	}
	_, network, err := net.ParseCIDR(item)
	if err != nil {
		return nil, fmt.Errorf("must be an IPv4 address or CIDR")
	}
	ip4 := network.IP.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("only IPv4 targets are supported")
	}
	ones, bits := network.Mask.Size()
	if bits != 32 {
		return nil, fmt.Errorf("only IPv4 targets are supported")
	}
	if ones < 24 {
		return nil, fmt.Errorf("prefixes shorter than /24 are not allowed")
	}
	count := 1 << (bits - ones)
	if !exclusion && count > MaxHosts {
		return nil, fmt.Errorf("prefixes shorter than /24 are not allowed")
	}
	base := uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])
	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		v := base + uint32(i)
		out = append(out, fmt.Sprintf("%d.%d.%d.%d", byte(v>>24), byte(v>>16), byte(v>>8), byte(v)))
	}
	return out, nil
}
