# SDN master plan

Project: SDN, a multi-site network infrastructure, fabric, and policy orchestration platform.

Repository: https://github.com/majdkh733-dev/SDN

Frontend host: Vercel, root directory `apps/web`.

This document is the working plan. The checklist in `docs/checklist.md` records what is implemented in this repository and what is still blocked.

## Product boundary

Operators plan policy, preview impact, obtain a separate approval, and then execute. Monitoring reads inventory, topology, desired state, observed state, and drift. Configuration changes go through a change request. The browser talks only to the API. It never opens a session to a switch, controller, or connector.

Demonstration data is produced by a deterministic simulator and is labeled in the API and the console. The OVN adapter is real code that shells to `ovn-nbctl`. No physical lab result is recorded for this deployment, so its support status is `implemented_unverified`. Other southbound protocols are visible and marked `not_implemented`.

## Phase 1 — Discovery, requirements, and architecture

### Journeys

1. Register a site and connector, then discover devices.
2. Select a topology device and inspect interfaces, capabilities, and health.
3. Create and validate a policy, then preview affected resources.
4. Submit the preview for approval.
5. Execute an approved change and follow its steps.
6. Read verification, partial execution, and rollback failure.
7. Compare desired and observed state and inspect drift.
8. Review backups and request an authorized restore.

### Tenancy

An organization is the customer boundary. Sites, connectors, devices, fabrics, tenants, policies, changes, and audit events belong to one organization. A network tenant is an isolation domain inside a fabric. It does not replace organization ownership. Every request handler sets the organization from the server session and queries with that organization id. PostgreSQL row-level security is a second check.

### Monitoring versus change

Reads (inventory, topology, drift, audit) require a read role. Writes that discover or back up require an operator, engineer, or administrator. Policy authoring and execution require an engineer or administrator. Approval requires an approver or administrator, and the requester cannot approve their own change. Restore requires a recovery operator or administrator. The server repeats these checks. The console hides actions the role cannot perform, and that hiding is not the control.

### Architecture

| Concern | Choice |
| --- | --- |
| Console | React, TypeScript, Vite in `apps/web` |
| API | Go modular monolith in `services/api` |
| Worker | Separate process in `services/worker` |
| Site connector | Outbound agent in `connectors/site-agent` |
| Database | PostgreSQL |
| Identity | OIDC authorization code with PKCE. Development also has an embedded provider. |
| Authorization | Server-side roles |
| Orchestration durability | PostgreSQL workflow rows, idempotent steps, advisory locks. Optional Temporal workflow when `TEMPORAL_ADDRESS` is set. |
| Secrets | Credential references. Values are not accepted by the API. Vault KV v2 client when `SECRET_BACKEND=vault`. |
| Southbound | Adapter boundary. Simulator plus OVN. |
| Coordination store | PostgreSQL epochs. etcd is not deployed. |

Decisions are in `docs/adr/`. The schema is in `docs/schema.md`. The HTTP contract is `packages/contracts/openapi.yaml`. Security and deployment are separate documents.

## Phase 2 — Milestones

| Milestone | Scope | State in this repository |
| --- | --- | --- |
| 1 Platform foundation | Auth, organization isolation, roles, layout, sites, inventory, seed | Implemented and covered by the HTTP journey test |
| 2 Network visibility | Device detail, interfaces, topology, capabilities, connector health, collection | Implemented for the simulator. OVN discovery returns the adapter error when `ovn-nbctl` is absent |
| 3 Policy planning | Versioned policies, validation, compatibility evidence, impact preview | Implemented. A newer version invalidates the previous preview |
| 4 Controlled execution | Change requests, approvals, author confirmation, durable execution, verification, backups, rollback, worker recovery | Implemented on the simulator. `TestWorkflowRecovery` covers confirmation, interrupted verification, and interrupted compensation |
| 5 Operational visibility | Desired versus observed, drift, workflow history, audit, command room | Implemented |

Adapter scope stays narrow. See `docs/adapters.md`.

## Phase 3 — Repository and backend

```
apps/web
services/api
services/worker
connectors/site-agent
connectors/ovn
packages/contracts
deploy
docs
.github/workflows
```

Local development uses PostgreSQL, migrations on API start, and `seed` for demonstration accounts. Docker Compose is in `deploy/docker-compose.yml` for hosts that have Docker. This development environment verified the API against a local PostgreSQL 16 instance.

## Phase 4 — Console

The console has the eleven navigation pages, English and Arabic with RTL, deep links for filters, and the eight journeys against the API. Network execution asks the operator to type the first eight characters of the plan hash and to check a confirmation box. The copy states that approval does not start execution. The server requires `confirm: true`, a matching plan hash, an approval of that hash, and a passing capability check before it sets `author_confirmed`. The Controllers page shows the orchestration driver, Temporal status, and secret backend returned by the API.

## Phase 5 — Hardening and launch

Automated checks cover organization isolation, self-approval denial, confirmation, stale previews, rollback failure, restore integrity, and site-agent discovery. GitHub Actions runs the Go tests, migration apply, frontend lint, typecheck, tests, production build, a secret scan, and `govulncheck`.

A frontend that runs the simulator is a demonstration release. Production launch is not complete until the API, external OIDC, PostgreSQL, connectors, and the required workflows are deployed and verified on the target network. That verification is not claimed here.

## Site connectors and Command Room

Connectors run inside the site and dial out. Enrollment is a 15-minute one-time token. The stored credential is revocable and is not returned again. Discovery jobs are leased to the assigned connector. An offline connector marks telemetry stale; it does not mark every device down. Identity is `site + serial`, then MAC, then IP, so the same private address in two sites stays two assets. Manual fields listed in `locked_fields` survive later imports. Navigation and theme are stored per organization. The Command Room reads inventory, verified topology, drift, and audit. Workload counts and availability percentages stay unavailable until those collectors exist. Details are in `docs/connectors.md`.

## Explicit gaps

- Temporal workflow code is implemented and covered by the SDK test suite. No Temporal cluster is deployed here, and `TEMPORAL_ADDRESS` is empty, so the driver that ran the recovery tests is PostgreSQL.
- etcd is not deployed. Fencing is an epoch row per execution domain. A database epoch does not by itself close an external device session.
- The Vault KV v2 client is implemented and tested against an in-process HTTP server. No Vault server is configured, so external secret storage is not deployed.
- `TestLabProbe` and `scripts/ovn-lab-check.sh` reported `implemented_unverified` because `ovn-nbctl` is not installed. No lab northbound result exists.
- Vercel project creation needs an account token. Production launch stays incomplete until the API, external OIDC, PostgreSQL, Vault, and connectors are deployed and verified.
- Split-origin cookies need `COOKIE_SAMESITE=none` and `SECURE_COOKIES=true`.
- Discovery was verified with a local TCP listener and the API test database. No physical switch, SNMP agent, or vendor API was available, so those collectors are not claimed as lab-verified.
- Helm, AWS, and Windows connector cards are visible and marked unavailable. They do not install anything.
