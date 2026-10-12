package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var defaultPorts = []int{22, 80, 443, 161}

// Method is one piece of evidence. A failed method is not proof that the host is absent.
type Method struct {
	Method string `json:"method"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
	Port   int    `json:"port,omitempty"`
}

// Observation is what a connector can support from a probe. Empty identity fields
// mean the method did not identify them.
type Observation struct {
	IP         string   `json:"ip"`
	Hostname   string   `json:"hostname,omitempty"`
	MAC        string   `json:"mac,omitempty"`
	OSVersion  string   `json:"os_version,omitempty"`
	Reachable  bool     `json:"reachable"`
	Methods    []Method `json:"methods"`
	Confidence string   `json:"confidence"`
	Explicit   bool     `json:"explicit"`
}

type Options struct {
	Timeout     time.Duration
	Ports       []int
	Community   string
	Concurrency int
	ICMP        func(ctx context.Context, ip string, timeout time.Duration) Method
	ARP         func() map[string]string
}

// Scan probes each target. Hosts expanded from a CIDR are omitted when nothing responds.
// An explicit address is always returned, including when every method is silent.
func Scan(ctx context.Context, targets []Target, opt Options) []Observation {
	if opt.Timeout <= 0 {
		opt.Timeout = 800 * time.Millisecond
	}
	if len(opt.Ports) == 0 {
		opt.Ports = defaultPorts
	}
	if opt.Concurrency <= 0 {
		opt.Concurrency = 8
	}
	if opt.ICMP == nil {
		opt.ICMP = icmpPing
	}
	if opt.ARP == nil {
		opt.ARP = readARP
	}
	arp := opt.ARP()
	sem := make(chan struct{}, opt.Concurrency)
	out := make([]Observation, len(targets))
	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, target Target) {
			defer wg.Done()
			defer func() { <-sem }()
			out[i] = probeOne(ctx, target, opt, arp[target.IP])
		}(i, target)
	}
	wg.Wait()
	var kept []Observation
	for _, item := range out {
		if item.Reachable || item.Explicit || item.Hostname != "" || item.MAC != "" {
			kept = append(kept, item)
		}
	}
	if kept == nil {
		kept = []Observation{}
	}
	return kept
}

func probeOne(ctx context.Context, target Target, opt Options, mac string) Observation {
	obs := Observation{IP: target.IP, Explicit: target.Explicit, Confidence: "low", Methods: []Method{}}
	if mac != "" {
		obs.MAC = mac
		obs.Methods = append(obs.Methods, Method{Method: "arp", OK: true, Detail: "neighbour table"})
		obs.Reachable = true
	} else {
		obs.Methods = append(obs.Methods, Method{Method: "arp", OK: false, Detail: "no neighbour entry"})
	}
	icmp := opt.ICMP(ctx, target.IP, opt.Timeout)
	obs.Methods = append(obs.Methods, icmp)
	if icmp.OK {
		obs.Reachable = true
	}
	for _, port := range opt.Ports {
		method := tcpProbe(ctx, target.IP, port, opt.Timeout)
		obs.Methods = append(obs.Methods, method)
		if method.OK {
			obs.Reachable = true
		}
	}
	if names, err := net.DefaultResolver.LookupAddr(ctx, target.IP); err == nil && len(names) > 0 {
		obs.Hostname = strings.TrimSuffix(names[0], ".")
		obs.Methods = append(obs.Methods, Method{Method: "dns", OK: true, Detail: obs.Hostname})
	} else {
		detail := "no reverse name"
		if err != nil {
			detail = "lookup failed"
		}
		obs.Methods = append(obs.Methods, Method{Method: "dns", OK: false, Detail: detail})
	}
	if opt.Community != "" {
		snmp := snmpGet(ctx, target.IP, opt.Community, opt.Timeout)
		obs.Methods = append(obs.Methods, snmp.methods...)
		if snmp.sysName != "" && obs.Hostname == "" {
			obs.Hostname = snmp.sysName
		}
		if snmp.sysDescr != "" {
			obs.OSVersion = snmp.sysDescr
			obs.Confidence = "medium"
		}
		if snmp.ok {
			obs.Reachable = true
		}
	}
	return obs
}

func tcpProbe(ctx context.Context, ip string, port int, timeout time.Duration) Method {
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return Method{Method: "tcp", OK: false, Port: port, Detail: "no response"}
	}
	_ = conn.Close()
	return Method{Method: "tcp", OK: true, Port: port, Detail: "connected"}
}

func icmpPing(ctx context.Context, ip string, timeout time.Duration) Method {
	if _, err := exec.LookPath("ping"); err != nil {
		return Method{Method: "icmp", OK: false, Detail: "ping is not installed"}
	}
	wait := int(timeout.Seconds())
	if wait < 1 {
		wait = 1
	}
	cmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", fmt.Sprint(wait), ip)
	if err := cmd.Run(); err != nil {
		return Method{Method: "icmp", OK: false, Detail: "no echo reply"}
	}
	return Method{Method: "icmp", OK: true, Detail: "echo reply"}
}

func readARP() map[string]string {
	raw, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		return map[string]string{}
	}
	return ParseARP(string(raw))
}

// ParseARP reads the Linux neighbour table. Incomplete entries are ignored.
func ParseARP(text string) map[string]string {
	out := map[string]string{}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		mac := strings.ToLower(fields[3])
		if mac == "00:00:00:00:00:00" || !strings.Contains(mac, ":") {
			continue
		}
		out[fields[0]] = mac
	}
	return out
}

// MustJSON is a test and agent helper.
func MustJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return []byte("[]")
	}
	return raw
}
