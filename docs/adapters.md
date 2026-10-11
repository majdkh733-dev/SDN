# Adapter support

The API calls adapters. The browser does not.

| Kind | Status | What the code does |
| --- | --- | --- |
| simulator | demo | Deterministic discovery and ACL apply. Verified by `TestPlatformJourneys` and the site-agent test. Devices are not forwarding traffic. |
| ovn | implemented_unverified | `connectors/ovn` runs `ovn-nbctl`. Unit tests use a fake runner. This environment has no OVN lab, so discovery reports that the adapter is unverified when the binary is missing. |
| netconf | not_implemented | Listed in the catalog. Create connector rejects it. |
| gnmi | not_implemented | Listed in the catalog. |
| vendor_api | not_implemented | Listed in the catalog. |
| openflow | not_implemented | Listed in the catalog. |
| gitops | not_implemented | Listed in the catalog. Nothing is deployed from Git. |

Simulator connectors default to in-process execution. Discovery can still be queued to a site agent with `?via=agent`. OVN connectors default to `site_agent` execution mode. The agent heartbeats, leases a task, and posts discovery over its outbound connection. The enrollment token is hashed at rest.
