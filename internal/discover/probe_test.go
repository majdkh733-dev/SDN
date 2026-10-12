package discover

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestParseARP(t *testing.T) {
	table := ParseARP("IP address       HW type     Flags       HW address            Mask     Device\n10.0.0.8         0x1         0x2         aa:bb:cc:dd:ee:ff     *        eth0\n10.0.0.9         0x1         0x0         00:00:00:00:00:00     *        eth0\n")
	if table["10.0.0.8"] != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("%#v", table)
	}
	if _, ok := table["10.0.0.9"]; ok {
		t.Fatal("incomplete ARP entry was kept")
	}
}

func TestScanTCPAndSilentExplicit(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	found := Scan(ctx, []Target{{IP: "127.0.0.1", Explicit: true}, {IP: "127.0.0.2", Explicit: false}}, Options{
		Timeout:     200 * time.Millisecond,
		Ports:       []int{port},
		Concurrency: 2,
		ICMP: func(context.Context, string, time.Duration) Method {
			return Method{Method: "icmp", OK: false, Detail: "no echo reply"}
		},
		ARP: func() map[string]string { return map[string]string{} },
	})
	if len(found) != 1 {
		t.Fatalf("only the explicit or responding host should remain, got %#v", found)
	}
	local := found[0]
	if !local.Reachable {
		t.Fatalf("tcp listener was not detected: %#v", local.Methods)
	}
	if local.OSVersion != "" || local.Confidence != "low" {
		t.Fatalf("tcp reachability invented identity: %#v", local)
	}
}

func TestSNMPRoundTrip(t *testing.T) {
	packet := encodeGetResponse("public", "DemoOS 1.0", "edge-1")
	descr, name, err := parseGet(packet)
	if err != nil {
		t.Fatal(err)
	}
	if descr != "DemoOS 1.0" || name != "edge-1" {
		t.Fatalf("descr %q name %q", descr, name)
	}
}

func encodeGetResponse(community, descr, name string) []byte {
	bindings := tlv(0x30, append(
		tlv(0x30, append(encodeOID([]int{1, 3, 6, 1, 2, 1, 1, 1, 0}), tlv(0x04, []byte(descr))...)),
		tlv(0x30, append(encodeOID([]int{1, 3, 6, 1, 2, 1, 1, 5, 0}), tlv(0x04, []byte(name))...))...,
	))
	pdu := append(tlv(0x02, []byte{1}), tlv(0x02, []byte{0})...)
	pdu = append(pdu, tlv(0x02, []byte{0})...)
	pdu = append(pdu, bindings...)
	body := append(tlv(0x02, []byte{1}), tlv(0x04, []byte(community))...)
	body = append(body, tlv(0xa2, pdu)...)
	return tlv(0x30, body)
}
