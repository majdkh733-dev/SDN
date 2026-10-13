# Adapter support

The API calls adapters. The browser does not.

| Kind | Status | Discover | Monitor | Read configuration | Write configuration | What the code does |
| --- | --- | --- | --- | --- | --- | --- |
| simulator | demo | yes, labeled simulator | stored health only | VLAN and ACL text in the simulator | VLAN and ACL apply through the change workflow | Deterministic discovery, ACL apply, and VLAN apply. Verified by `TestPlatformJourneys` and `TestVLANChangeVerifyAndDrift`. Devices are not forwarding traffic. This is not production health. |
| site connector | visibility | ICMP, TCP, ARP, DNS, optional SNMP v2c on the allow-list | heartbeat and job result | no device configuration text | no | Outbound collector. It is not a controller. |
| ovn | implemented_unverified | when `ovn-nbctl` is present | no continuous counters | northbound logical state | `ovn-nbctl` changes when the binary exists | `connectors/ovn` runs `ovn-nbctl`. Unit tests use a fake runner. No northbound database was listed here. |
| netconf | not_implemented | no | no | no | no | Listed in the catalog. Create connector rejects it. |
| gnmi | not_implemented | no | no | no | no | Listed in the catalog. |
| vendor_api | not_implemented | no | no | no | no | Listed in the catalog. Cisco Catalyst Center, HPE Aruba Central, and Juniper Apstra are not adapters in this repository. |
| openflow | not_implemented | no | no | no | no | Listed in the catalog. |
| gitops | not_implemented | no | no | no | no | Nothing is deployed from Git. |

A topology dashboard is not an SDN control plane. Programmable change in this repository is the simulator VLAN and ACL path, plus the unverified OVN path. Desired state is the policy version on a change request. Observed state is the last collection. Drift is the difference after both exist. Rollback is only the workflow compensation implemented for the simulator, and a failed compensation stays `rollback_failed`.

Simulator connectors default to in-process execution. Discovery can still be queued to a site agent with `?via=agent`. OVN connectors default to `site_agent` execution mode. The agent heartbeats, leases a task, and posts discovery over its outbound connection. The enrollment token is hashed at rest.
