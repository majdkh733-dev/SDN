# Operations platform

This document records what the current repository can actually do. Support that is not listed as implemented has not been executed against a device.

## Acceptance workflow

Discover a host, import it, inspect interfaces, retrieve configuration, propose a change, approve it, apply it, verify it, then detect a later difference.

| Step | What runs | Limit |
| --- | --- | --- |
| Discover | Site connector probes an allow-listed IPv4 target | At most 256 addresses. Prefixes shorter than /24 are rejected. |
| Import | Identity is site plus serial, then MAC, then IP | Locked manual fields are kept. Ambiguous matches are not merged. |
| Interfaces | Rows collected with the device | A discovery-only host may have no interface rows. |
| Configuration read | Simulator lab adapter renders a text config | SSH, NETCONF, RESTCONF, and gNMI are not implemented. |
| Change | VLAN proposal creates a validated change with a generated stanza and a baseline checksum | Only `data_origin=simulator` can be written. |
| Approve | A different user approves the exact plan hash | The author cannot approve their own change. |
| Execute | Author confirms the same hash. The worker applies the simulator VLAN row | If the observed checksum changed, execution fails and does not write. |
| Verify | The simulator VLAN row must exist | This is not a traffic probe. Rollback deletes that row. |
| Drift | A later collection that differs from the saved baseline opens a config finding | No person is blamed unless an audit event exists. |

`TestVLANChangeVerifyAndDrift` covers stale-baseline rejection, a successful VLAN apply, cross-organization isolation, config drift, and service-account revocation.

## Adapter matrix

`GET /api/v1/adapters` returns the same matrix as the Integrations page.

| Adapter | Discovery | Monitoring | Config read | Config write | Status |
| --- | --- | --- | --- | --- | --- |
| simulator | yes | counters are not collected | yes | VLAN and ACL | demo, covered by tests |
| site connector | ICMP, ARP, TCP, DNS, optional SNMPv2c | no | no | no | implemented for discovery |
| ovn | code calls `ovn-nbctl` | no | unverified | unverified | no lab result recorded |
| ssh, netconf, restconf, gnmi, vendor API | no | no | no | no | not implemented |

SNMPv3, host-key checks, and TLS device sessions are not implemented because those clients do not exist. Do not point the connector at production devices to try them.

## Navigation

The default sidebar groups are Dashboard, Network, Monitoring, Configuration, Policies, Administration, and Reports. A saved menu that still has the previous eight group ids is replaced with this catalog. A custom group is kept. A saved menu that is missing a required page is filled in without discarding a custom label or order. Routes for firmware, QoS, endpoints, and the duplicate fabric, provisioning, validation, and assurance pages stay available, and they are not added back into the default menu. Reset restores the default. Hiding a page does not change API authorization.

Old paths still work: `/inventory`, `/desired` (redirects to `/drift`), `/devices` (redirects to `/inventory`), `/platform` (redirects to `/controllers`).

Organizations are the SaaS boundary. Network tenants and VRFs are rows in `tenants` and are not organizations.

## Configuration versus disaster recovery

`device_config_revisions` stores simulator running-config text. `configuration_backups` stores the platform change manifest. Restoring a platform backup does not push a device config. Device rollback for a VLAN change deletes the simulator row. There is no automatic rollback for an adapter that cannot write.

## Reports

Availability is a count of devices whose current health is `healthy` or `up`, divided only as a snapshot. No historical percentage is calculated. Firmware end-of-support dates and vulnerability scores are not invented.

## Service accounts and webhooks

An administrator can create a service account. The token is returned once and stored as a SHA-256 hash. Revocation rejects the token. A webhook row can subscribe to `change.succeeded`. Delivery errors are stored and do not fail the change. No ITSM or SIEM product is bundled.

## Diagnostics

Ping and reverse DNS are queued to the connector that owns the site. The API does not dial the target.

## Capacity assumptions

One API process and one PostgreSQL database. Discovery jobs are leased for two minutes to a single connector. A scan is capped at 256 addresses. The connector polls about every 15 seconds, backing off to 30 seconds. This has not been load-tested beyond the automated suite. Do not treat those limits as a cluster capacity number.

## API additions

These routes are implemented in the API server. They are not yet copied into `packages/contracts/openapi.yaml`.

| Method | Path | Who |
| --- | --- | --- |
| GET | `/api/v1/adapters` | reader |
| POST | `/api/v1/provisioning/vlan` | author |
| GET | `/api/v1/interfaces` | reader |
| GET | `/api/v1/endpoints` | reader |
| GET | `/api/v1/vlans` | reader |
| GET | `/api/v1/config-revisions` | reader |
| POST | `/api/v1/devices/{id}/config/collect` | operator |
| POST | `/api/v1/devices/{id}/lab/overlay` | operator, simulator only |
| POST | `/api/v1/diagnostics` | operator |
| GET | `/api/v1/reports/availability` | reader |
| GET, POST | `/api/v1/service-accounts` | administrator |
| POST | `/api/v1/service-accounts/{id}/revoke` | administrator |

Existing change routes still approve and execute: `POST /api/v1/changes/{id}/approve` and `POST /api/v1/changes/{id}/execute`.

## Deployment

Start the API so migration `006_operations.sql` applies. Connector install remains `docs/connectors.md`. Scheduled discovery still needs `services/worker`. Inline change execution is the local default (`INLINE_WORKFLOWS=true`). No production deploy is performed by this change.
