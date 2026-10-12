# Implementation checklist

Status reflects the code in this repository, not a production deployment.

## Phase 1

- [x] User journeys and the monitoring versus change boundary — `docs/master-plan.md`
- [x] Architecture decisions — `docs/adr/`
- [x] Schema, ownership, indexes, lifecycle, retention — `docs/schema.md` and `internal/migrate/sql/`
- [x] OpenAPI contract — `packages/contracts/openapi.yaml`
- [x] Security model — `docs/security.md`
- [x] Deployment architecture — `docs/deployment.md`

## Phase 2

- [x] Milestone 1 foundation
- [x] Milestone 2 simulator visibility
- [x] Milestone 3 policy preview
- [x] Milestone 4 controlled execution on the simulator
- [x] Milestone 5 operational views
- [x] Simulator adapter
- [ ] OVN adapter verified in a lab (`TestLabProbe` and `scripts/ovn-lab-check.sh`: `implemented_unverified`, `ovn-nbctl` is not installed)
- [x] Other vendors listed as not implemented

## Phase 3

- [x] Repository layout
- [x] Migrations and seed
- [x] OIDC authorization code and PKCE, plus development password login
- [x] Server-side roles and organization isolation
- [x] Inventory, topology, policy, change, backup, and audit APIs
- [x] Connector enrollment, credential references, health, and outbound task lease
- [x] Workflow persistence, idempotent steps, cancellation, failure reporting, and worker recovery (`TestWorkflowRecovery`)
- [x] Author confirmation separate from approval
- [x] GitHub Actions for Go, web, migrations, and security checks
- [x] `.env.example` files
- [x] Docker Compose and container images
- [x] Temporal workflow and SDK test suite (`TestExecuteChangeUsesTheSuppliedIdentifiers`). No cluster is running in this environment
- [ ] Temporal cluster
- [x] Vault KV v2 client tested with an HTTP test server. Production config refuses memory and file backends
- [ ] External Vault deployment

## Phase 4

- [x] Eleven navigation pages
- [x] Eight simulator journeys in the console
- [x] English and Arabic RTL
- [x] Explicit execution confirmation and stale-preview rejection. Approval alone does not start execution
- [x] Partial execution and rollback failure shown on the change
- [ ] Lab adapter exercised from the console

## Phase 5

- [x] Organization isolation test
- [x] Approval and confirmation tests
- [x] Stale plan test
- [x] Rollback failure test
- [x] Backup checksum and pending restore test
- [x] Site-agent discover test
- [x] Frontend lint, typecheck, unit tests, and production build
- [ ] Vercel deployment URL
- [ ] Production launch (API, external OIDC, storage, connectors, and workflows verified on the target network)

## Site connectors and Command Room

- [x] One-time enrollment, revocable connector credential, heartbeat, and site-scoped job lease
- [x] Docker, Compose, and Linux install paths; other platforms labeled unavailable
- [x] Bounded local discovery (ICMP, ARP, TCP, DNS, optional SNMP) and inventory import without duplicate identity keys
- [x] Company navigation and theme preferences
- [x] Command Room topology, incidents, and Policy Studio scope handoff
- [x] Connector lifecycle test (`TestConnectorDiscoveryLifecycle`) and agent probe test
- [ ] Physical device or vendor API collection outside this environment

## Operations platform

- [x] Default navigation matches the operations groups, and older saved trees keep custom order while gaining missing pages
- [x] Adapter support matrix (`GET /api/v1/adapters`)
- [x] Simulator VLAN change: baseline check, approval, verify, rollback row delete, and later config drift (`TestVLANChangeVerifyAndDrift`)
- [x] Service account token issue and revocation
- [x] Connector ping and DNS diagnostics queued as agent tasks
- [ ] SSH, NETCONF, RESTCONF, gNMI, SNMPv3, and vendor configuration write
- [ ] Historical availability, flow telemetry, and firmware lifecycle data
- [ ] Live switch lab for the VLAN workflow

## Reference topology

- [x] UTEC draw.io source recovered from the PNG `mxfile` chunk and rendered with the diagrams.net viewer and its stencil files
- [x] Device and link selection, inventory binding, and saved diagram versions (`TestTopologyDiagramBindingIsolation`)
- [ ] Stylized logo text for SS and Al Kharj is artwork, not a separate text cell; OCR did not read those two wordmarks reliably
- [ ] Reference symbols are not pre-bound to inventory devices
