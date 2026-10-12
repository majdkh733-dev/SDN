package adapter

// Support is one cell in the adapter matrix. Status is implemented only when
// this repository can execute the operation. implemented_unverified means the
// code exists and has not been exercised against a live system.
type Support struct {
	Adapter  string `json:"adapter"`
	Status   string `json:"status"`
	Note     string `json:"note"`
	Read     bool   `json:"config_read"`
	Write    bool   `json:"config_write"`
	Monitor  bool   `json:"monitoring"`
	Discover bool   `json:"discovery"`
}

func Matrix() []Support {
	return []Support{
		{Adapter: "simulator", Status: "demo", Discover: true, Monitor: true, Read: true, Write: true, Note: "In-process lab adapter. VLAN and ACL changes are verified against simulator tables, not a forwarding device."},
		{Adapter: "site", Status: "implemented", Discover: true, Monitor: false, Read: false, Write: false, Note: "Outbound connector. Reachability, ARP, DNS, and optional SNMP identify hosts. It does not change device configuration."},
		{Adapter: "ovn", Status: "implemented_unverified", Discover: true, Monitor: false, Read: true, Write: true, Note: "Shells to ovn-nbctl. No northbound lab result is recorded in this environment."},
		{Adapter: "snmp", Status: "partial", Discover: true, Monitor: false, Read: true, Write: false, Note: "Optional SNMPv2c read of sysName and sysDescr during a connector scan. SNMPv3, counters, and writes are not implemented."},
		{Adapter: "ssh", Status: "not_implemented", Note: "SSH collection is not implemented. Host-key verification is therefore not available."},
		{Adapter: "netconf", Status: "not_implemented", Note: "NETCONF is listed only. No session is opened."},
		{Adapter: "restconf", Status: "not_implemented", Note: "RESTCONF is not implemented."},
		{Adapter: "gnmi", Status: "not_implemented", Note: "gNMI/OpenConfig is not implemented."},
		{Adapter: "vendor_api", Status: "not_implemented", Note: "No vendor write API is enrolled."},
	}
}
