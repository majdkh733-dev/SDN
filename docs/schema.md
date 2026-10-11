# Schema

Source of truth: `internal/migrate/sql/001_schema.sql` and `002_rls.sql`.

Organization is the owner of every tenant table. A fabric tenant is a network isolation domain and has its own `organization_id`. Composite unique keys `(organization_id, id)` back composite foreign keys so a row cannot point at another organization's parent.

Lifecycle for sites, devices, fabrics, tenants, and segments is `active`, then `disabled`, then `archived`. Archived rows stay for audit and are not execution targets.

Change status moves `validated` to `approved`, `rejected`, or `cancelled`; `approved` to `queued`; `queued` to `applying`; `applying` to `verifying`, `partial`, `failed`, `unknown`, or `cancelled`; `verifying` to `succeeded`, `failed`, or `unknown`; `partial` to `compensating`; `compensating` to `compensated` or `rollback_failed`. An `unknown` outcome is retried by inspecting the simulator ACL record before writing again.

Retention is `organizations.retention_days` (30 to 3650, default 365). `POST /api/v1/maintenance/retention` deletes aged audit events and observed snapshots, and deletes aged configuration backups except the newest five and any backup referenced by a restore.

```mermaid
erDiagram
  organizations ||--o{ memberships : has
  users ||--o{ memberships : joins
  organizations ||--o{ sites : owns
  sites ||--o{ connectors : hosts
  connectors ||--o{ devices : reports
  devices ||--o{ interfaces : has
  interfaces ||--o{ topology_links : connects
  devices ||--o{ capability_snapshots : advertises
  organizations ||--o{ fabrics : owns
  fabrics ||--o{ tenants : isolates
  tenants ||--o{ segments : contains
  tenants ||--o{ policies : governs
  policies ||--o{ policy_versions : versions
  policy_versions ||--o{ change_requests : proposes
  change_requests ||--o{ approvals : reviews
  change_requests ||--o{ workflow_runs : runs
  workflow_runs ||--o{ execution_steps : steps
  change_requests ||--o{ verification_results : checks
  organizations ||--o{ desired_states : wants
  devices ||--o{ observed_states : reports
  devices ||--o{ drift_findings : differs
  organizations ||--o{ configuration_backups : saves
  configuration_backups ||--o{ restore_runs : restores
  organizations ||--o{ audit_events : records
  connectors ||--o{ agent_tasks : queues
```

Indexes follow the access paths used by the API: organization plus lifecycle or status, device lookup, and time-ordered audit, drift, and backups. `execution_steps` is unique on `(change_request_id, operation_key)` so a retry does not insert a second step. `idempotency_keys` stores the change-create response for a repeated `Idempotency-Key`.
