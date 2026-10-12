package discover

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

type snmpResult struct {
	ok       bool
	sysDescr string
	sysName  string
	methods  []Method
}

func snmpGet(ctx context.Context, ip, community string, timeout time.Duration) snmpResult {
	result := snmpResult{}
	payload := encodeGet(community, [][]int{
		{1, 3, 6, 1, 2, 1, 1, 1, 0},
		{1, 3, 6, 1, 2, 1, 1, 5, 0},
	})
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "udp", ip+":161")
	if err != nil {
		result.methods = append(result.methods, Method{Method: "snmp", OK: false, Port: 161, Detail: "no response"})
		return result
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(payload); err != nil {
		result.methods = append(result.methods, Method{Method: "snmp", OK: false, Port: 161, Detail: "request failed"})
		return result
	}
	buf := make([]byte, 1500)
	n, err := conn.Read(buf)
	if err != nil {
		result.methods = append(result.methods, Method{Method: "snmp", OK: false, Port: 161, Detail: "no response"})
		return result
	}
	descr, name, err := parseGet(buf[:n])
	if err != nil {
		result.methods = append(result.methods, Method{Method: "snmp", OK: false, Port: 161, Detail: "response was not understood"})
		return result
	}
	result.ok = true
	result.sysDescr = descr
	result.sysName = name
	detail := "sysDescr and sysName"
	if descr == "" && name == "" {
		detail = "response contained no sysDescr or sysName"
	}
	result.methods = append(result.methods, Method{Method: "snmp", OK: true, Port: 161, Detail: detail})
	return result
}

func encodeGet(community string, oids [][]int) []byte {
	var bindings []byte
	for i, oid := range oids {
		vb := append(encodeOID(oid), byte(0x05), byte(0x00))
		vb = tlv(0x30, vb)
		bindings = append(bindings, vb...)
		_ = i
	}
	pdu := append([]byte{0x02, 0x01, 0x01}, tlv(0x02, []byte{1})...)
	pdu = append(pdu, tlv(0x02, []byte{0})...)
	pdu = append(pdu, tlv(0x02, []byte{0})...)
	pdu = append(pdu, tlv(0x30, bindings)...)
	body := append(tlv(0x02, []byte{1}), tlv(0x04, []byte(community))...)
	body = append(body, tlv(0xa0, pdu)...)
	return tlv(0x30, body)
}

func encodeOID(parts []int) []byte {
	var body []byte
	if len(parts) >= 2 {
		body = append(body, byte(parts[0]*40+parts[1]))
		parts = parts[2:]
	}
	for _, part := range parts {
		body = append(body, encodeBase128(part)...)
	}
	return tlv(0x06, body)
}

func encodeBase128(v int) []byte {
	if v < 0 {
		v = 0
	}
	var rev []byte
	rev = append(rev, byte(v&0x7f))
	v >>= 7
	for v > 0 {
		rev = append(rev, byte(0x80|(v&0x7f)))
		v >>= 7
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

func tlv(tag byte, value []byte) []byte {
	out := []byte{tag}
	out = append(out, encodeLen(len(value))...)
	return append(out, value...)
}

func encodeLen(n int) []byte {
	if n < 128 {
		return []byte{byte(n)}
	}
	raw := make([]byte, 4)
	binary.BigEndian.PutUint32(raw, uint32(n))
	i := 0
	for i < len(raw)-1 && raw[i] == 0 {
		i++
	}
	raw = raw[i:]
	return append([]byte{0x80 | byte(len(raw))}, raw...)
}

func parseGet(packet []byte) (string, string, error) {
	_, value, err := readTLV(packet)
	if err != nil {
		return "", "", err
	}
	rest := value
	for len(rest) > 0 {
		tag, inner, next, err := readTLVRest(rest)
		if err != nil {
			return "", "", err
		}
		if tag == 0xa2 || tag == 0xa0 {
			return stringsFromPDU(inner)
		}
		rest = next
	}
	return "", "", fmt.Errorf("snmp pdu missing")
}

func stringsFromPDU(pdu []byte) (string, string, error) {
	rest := pdu
	var descr, name string
	for len(rest) > 0 {
		tag, inner, next, err := readTLVRest(rest)
		if err != nil {
			return descr, name, err
		}
		if tag == 0x30 {
			d, n := bindings(inner)
			if d != "" {
				descr = d
			}
			if n != "" {
				name = n
			}
		}
		rest = next
	}
	return descr, name, nil
}

func bindings(seq []byte) (string, string) {
	rest := seq
	var descr, name string
	for len(rest) > 0 {
		tag, inner, next, err := readTLVRest(rest)
		if err != nil || tag != 0x30 {
			break
		}
		oid, val := binding(inner)
		switch oid {
		case "1.3.6.1.2.1.1.1.0":
			descr = val
		case "1.3.6.1.2.1.1.5.0":
			name = val
		}
		rest = next
	}
	return descr, name
}

func binding(seq []byte) (string, string) {
	rest := seq
	oid := ""
	val := ""
	for len(rest) > 0 {
		tag, inner, next, err := readTLVRest(rest)
		if err != nil {
			break
		}
		if tag == 0x06 {
			oid = decodeOID(inner)
		}
		if tag == 0x04 {
			val = string(inner)
		}
		rest = next
	}
	return oid, val
}

func decodeOID(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	parts := []int{int(body[0]) / 40, int(body[0]) % 40}
	value := 0
	for _, b := range body[1:] {
		value = (value << 7) | int(b&0x7f)
		if b&0x80 == 0 {
			parts = append(parts, value)
			value = 0
		}
	}
	out := ""
	for i, part := range parts {
		if i > 0 {
			out += "."
		}
		out += fmt.Sprint(part)
	}
	return out
}

func readTLV(packet []byte) (byte, []byte, error) {
	tag, value, _, err := readTLVRest(packet)
	return tag, value, err
}

func readTLVRest(packet []byte) (byte, []byte, []byte, error) {
	if len(packet) < 2 {
		return 0, nil, nil, fmt.Errorf("truncated")
	}
	tag := packet[0]
	n, used, err := decodeLen(packet[1:])
	if err != nil {
		return 0, nil, nil, err
	}
	start := 1 + used
	if len(packet) < start+n {
		return 0, nil, nil, fmt.Errorf("truncated")
	}
	return tag, packet[start : start+n], packet[start+n:], nil
}

func decodeLen(b []byte) (int, int, error) {
	if len(b) == 0 {
		return 0, 0, fmt.Errorf("truncated")
	}
	if b[0] < 128 {
		return int(b[0]), 1, nil
	}
	count := int(b[0] & 0x7f)
	if count == 0 || len(b) < 1+count {
		return 0, 0, fmt.Errorf("truncated")
	}
	n := 0
	for i := 0; i < count; i++ {
		n = (n << 8) | int(b[1+i])
	}
	return n, 1 + count, nil
}
