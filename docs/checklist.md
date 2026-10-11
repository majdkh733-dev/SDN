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
