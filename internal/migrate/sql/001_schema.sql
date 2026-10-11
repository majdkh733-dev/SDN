-- SDN platform schema.
-- Organization is the customer boundary. Network tenants are isolation domains
-- inside one organization and never replace organization ownership.
-- Retention: organizations.retention_days governs audit events, observed
-- snapshots, and aged backups. The newest five backups per organization are
-- kept even when they are older than the retention window.
-- Lifecycle: active -> disabled -> archived. Archived rows stay for audit and
-- are not reused as execution targets.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE organizations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    slug text NOT NULL UNIQUE,
    retention_days integer NOT NULL DEFAULT 365 CHECK (retention_days >= 30 AND retention_days <= 3650),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL,
    display_name text NOT NULL,
    oidc_subject text,
    password_hash text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_lower_idx ON users (lower(email));
CREATE UNIQUE INDEX users_oidc_subject_idx ON users (oidc_subject) WHERE oidc_subject IS NOT NULL;

CREATE TABLE memberships (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id),
    user_id uuid NOT NULL REFERENCES users (id),
    role text NOT NULL CHECK (role IN (
        'viewer', 'operator', 'engineer', 'approver', 'administrator', 'recovery_operator'
    )),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, user_id)
);

CREATE INDEX memberships_user_idx ON memberships (user_id);

CREATE TABLE sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users (id),
    organization_id uuid NOT NULL REFERENCES organizations (id),
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_idx ON sessions (user_id);

CREATE TABLE oidc_auth_codes (
    code_hash text PRIMARY KEY,
    client_id text NOT NULL,
    user_id uuid NOT NULL REFERENCES users (id),
    organization_id uuid NOT NULL REFERENCES organizations (id),
    code_challenge text NOT NULL,
    redirect_uri text NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz
);

CREATE TABLE sites (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id),
    name text NOT NULL,
    location text NOT NULL DEFAULT '',
    execution_domain text NOT NULL DEFAULT 'default',
    lifecycle text NOT NULL DEFAULT 'active' CHECK (lifecycle IN ('active', 'disabled', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name),
    UNIQUE (organization_id, id)
);

CREATE INDEX sites_org_idx ON sites (organization_id, lifecycle);

CREATE TABLE connectors (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id),
    site_id uuid NOT NULL,
    name text NOT NULL,
    adapter text NOT NULL CHECK (adapter IN ('simulator', 'ovn', 'netconf', 'gnmi', 'vendor_api')),
    version text NOT NULL DEFAULT '0.1.0',
    lifecycle text NOT NULL DEFAULT 'active' CHECK (lifecycle IN ('active', 'disabled', 'archived')),
    health text NOT NULL DEFAULT 'unknown' CHECK (health IN ('unknown', 'healthy', 'degraded', 'disconnected')),
    execution_mode text NOT NULL DEFAULT 'site_agent' CHECK (execution_mode IN ('inprocess', 'site_agent')),
    last_heartbeat_at timestamptz,
    credential_ref text NOT NULL DEFAULT '',
    support_status text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, id),
    UNIQUE (organization_id, name),
    FOREIGN KEY (organization_id, site_id) REFERENCES sites (organization_id, id)
);

CREATE INDEX connectors_org_site_idx ON connectors (organization_id, site_id);

CREATE TABLE connector_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    connector_id uuid NOT NULL,
    token_hash text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, connector_id) REFERENCES connectors (organization_id, id)
);

CREATE TABLE devices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    site_id uuid NOT NULL,
    connector_id uuid,
    name text NOT NULL,
    model text NOT NULL DEFAULT '',
    os_version text NOT NULL DEFAULT '',
    role text NOT NULL DEFAULT 'switch',
    management_endpoint text NOT NULL DEFAULT '',
    lifecycle text NOT NULL DEFAULT 'active' CHECK (lifecycle IN ('active', 'disabled', 'archived')),
    health text NOT NULL DEFAULT 'unknown' CHECK (health IN ('unknown', 'healthy', 'degraded', 'down')),
    data_origin text NOT NULL DEFAULT 'simulator' CHECK (data_origin IN ('simulator', 'ovn', 'seed')),
    support_note text NOT NULL DEFAULT '',
    fail_next_write boolean NOT NULL DEFAULT false,
    fail_next_rollback boolean NOT NULL DEFAULT false,
    uncertain_next_write boolean NOT NULL DEFAULT false,
    uncertain_committed boolean NOT NULL DEFAULT false,
    observed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, site_id) REFERENCES sites (organization_id, id),
    FOREIGN KEY (organization_id, connector_id) REFERENCES connectors (organization_id, id)
);

CREATE INDEX devices_org_site_idx ON devices (organization_id, site_id, lifecycle);

CREATE TABLE interfaces (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    device_id uuid NOT NULL,
    name text NOT NULL,
    mac text NOT NULL DEFAULT '',
    admin_up boolean NOT NULL DEFAULT true,
    oper_up boolean NOT NULL DEFAULT true,
    speed_mbps integer NOT NULL DEFAULT 0,
    addresses jsonb NOT NULL DEFAULT '[]'::jsonb,
    UNIQUE (device_id, name),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, device_id) REFERENCES devices (organization_id, id)
);

CREATE INDEX interfaces_device_idx ON interfaces (device_id);

CREATE TABLE topology_links (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    site_id uuid NOT NULL,
    a_interface_id uuid NOT NULL,
    b_interface_id uuid NOT NULL,
    source text NOT NULL DEFAULT 'discovered',
    utilization_pct numeric(5, 2) NOT NULL DEFAULT 0 CHECK (utilization_pct >= 0 AND utilization_pct <= 100),
    observed_at timestamptz NOT NULL DEFAULT now(),
    CHECK (a_interface_id <> b_interface_id),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, site_id) REFERENCES sites (organization_id, id),
    FOREIGN KEY (organization_id, a_interface_id) REFERENCES interfaces (organization_id, id),
    FOREIGN KEY (organization_id, b_interface_id) REFERENCES interfaces (organization_id, id)
);

CREATE INDEX topology_links_org_idx ON topology_links (organization_id, observed_at DESC);

CREATE TABLE capability_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    device_id uuid NOT NULL,
    capability text NOT NULL,
    supported boolean NOT NULL,
    evidence text NOT NULL DEFAULT '',
    connector_version text NOT NULL DEFAULT '',
    discovered_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (organization_id, device_id) REFERENCES devices (organization_id, id)
);

CREATE INDEX capability_device_idx ON capability_snapshots (device_id, discovered_at DESC);
CREATE INDEX capability_org_idx ON capability_snapshots (organization_id, capability);

CREATE TABLE fabrics (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id),
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    lifecycle text NOT NULL DEFAULT 'active' CHECK (lifecycle IN ('active', 'disabled', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name),
    UNIQUE (organization_id, id)
);

CREATE TABLE tenants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    fabric_id uuid NOT NULL,
    name text NOT NULL,
    isolation_domain text NOT NULL DEFAULT '',
    lifecycle text NOT NULL DEFAULT 'active' CHECK (lifecycle IN ('active', 'disabled', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, fabric_id) REFERENCES fabrics (organization_id, id)
);

CREATE INDEX tenants_org_idx ON tenants (organization_id, fabric_id);

CREATE TABLE segments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    name text NOT NULL,
    cidr text NOT NULL DEFAULT '',
    vlan integer,
    vni integer,
    lifecycle text NOT NULL DEFAULT 'active' CHECK (lifecycle IN ('active', 'disabled', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, tenant_id) REFERENCES tenants (organization_id, id)
);

CREATE INDEX segments_org_idx ON segments (organization_id, tenant_id);

CREATE TABLE policies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    name text NOT NULL,
    lifecycle text NOT NULL DEFAULT 'active' CHECK (lifecycle IN ('active', 'disabled', 'archived')),
    created_by uuid REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, tenant_id) REFERENCES tenants (organization_id, id)
);

CREATE INDEX policies_org_idx ON policies (organization_id, tenant_id);

CREATE TABLE policy_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    policy_id uuid NOT NULL,
    revision integer NOT NULL CHECK (revision > 0),
    spec jsonb NOT NULL,
    validation_status text NOT NULL CHECK (validation_status IN ('draft', 'valid', 'invalid')),
    validation_errors jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_by uuid REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (policy_id, revision),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, policy_id) REFERENCES policies (organization_id, id)
);

CREATE INDEX policy_versions_policy_idx ON policy_versions (policy_id, revision DESC);

CREATE TABLE desired_states (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id),
    scope text NOT NULL,
    revision integer NOT NULL CHECK (revision > 0),
    document jsonb NOT NULL,
    source_change_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, scope, revision),
    UNIQUE (organization_id, id)
);

CREATE INDEX desired_states_org_idx ON desired_states (organization_id, scope, revision DESC);

CREATE TABLE observed_states (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    device_id uuid NOT NULL,
    document jsonb NOT NULL,
    freshness text NOT NULL DEFAULT 'fresh' CHECK (freshness IN ('fresh', 'stale', 'failed')),
    collector text NOT NULL,
    observed_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, device_id) REFERENCES devices (organization_id, id)
);

CREATE INDEX observed_states_device_idx ON observed_states (device_id, observed_at DESC);

CREATE TABLE drift_findings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    device_id uuid NOT NULL,
    desired_state_id uuid,
    observed_state_id uuid,
    difference jsonb NOT NULL,
    disposition text NOT NULL DEFAULT 'open' CHECK (disposition IN ('open', 'accepted', 'remediated')),
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (organization_id, device_id) REFERENCES devices (organization_id, id),
    FOREIGN KEY (organization_id, desired_state_id) REFERENCES desired_states (organization_id, id),
    FOREIGN KEY (organization_id, observed_state_id) REFERENCES observed_states (organization_id, id)
);

CREATE INDEX drift_org_idx ON drift_findings (organization_id, disposition, created_at DESC);

CREATE TABLE change_requests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id),
    policy_version_id uuid NOT NULL,
    idempotency_key text,
    plan_hash text NOT NULL,
    impact jsonb NOT NULL,
    status text NOT NULL CHECK (status IN (
        'validated', 'approved', 'rejected', 'queued', 'applying', 'verifying',
        'succeeded', 'partial', 'failed', 'unknown', 'compensating', 'compensated',
        'rollback_failed', 'cancelled'
    )),
    requested_by uuid NOT NULL REFERENCES users (id),
    execution_epoch bigint NOT NULL DEFAULT 0,
    cancel_requested boolean NOT NULL DEFAULT false,
    error_detail text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, policy_version_id) REFERENCES policy_versions (organization_id, id)
);

CREATE UNIQUE INDEX change_requests_idempotency_idx
    ON change_requests (organization_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX change_requests_org_status_idx ON change_requests (organization_id, status, created_at DESC);

CREATE TABLE approvals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    change_request_id uuid NOT NULL,
    plan_hash text NOT NULL,
    reviewer_id uuid NOT NULL REFERENCES users (id),
    decision text NOT NULL CHECK (decision IN ('approved', 'rejected')),
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (organization_id, change_request_id) REFERENCES change_requests (organization_id, id)
);

CREATE INDEX approvals_change_idx ON approvals (change_request_id, created_at DESC);

CREATE TABLE workflow_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    change_request_id uuid NOT NULL,
    status text NOT NULL,
    attempt integer NOT NULL DEFAULT 0,
    last_error text NOT NULL DEFAULT '',
    started_at timestamptz,
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, change_request_id) REFERENCES change_requests (organization_id, id)
);

CREATE INDEX workflow_runs_status_idx ON workflow_runs (status, created_at);

CREATE TABLE execution_steps (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    workflow_run_id uuid NOT NULL,
    change_request_id uuid NOT NULL,
    device_id uuid,
    operation_key text NOT NULL,
    status text NOT NULL CHECK (status IN (
        'pending', 'running', 'succeeded', 'failed', 'unknown', 'skipped',
        'compensated', 'compensation_failed'
    )),
    authority_epoch bigint NOT NULL DEFAULT 0,
    attempt integer NOT NULL DEFAULT 0,
    result jsonb NOT NULL DEFAULT '{}'::jsonb,
    observed jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (change_request_id, operation_key),
    FOREIGN KEY (organization_id, change_request_id) REFERENCES change_requests (organization_id, id),
    FOREIGN KEY (organization_id, workflow_run_id) REFERENCES workflow_runs (organization_id, id),
    FOREIGN KEY (organization_id, device_id) REFERENCES devices (organization_id, id)
);

CREATE INDEX execution_steps_change_idx ON execution_steps (change_request_id, created_at);

CREATE TABLE verification_results (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id),
    change_request_id uuid,
    scope text NOT NULL,
    assertions jsonb NOT NULL,
    result text NOT NULL CHECK (result IN ('passed', 'failed', 'unknown')),
    evidence text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX verification_org_idx ON verification_results (organization_id, created_at DESC);

CREATE TABLE configuration_backups (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id),
    scope text NOT NULL,
    manifest jsonb NOT NULL,
    checksum text NOT NULL,
    integrity text NOT NULL DEFAULT 'valid' CHECK (integrity IN ('valid', 'mismatch')),
    created_by uuid REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, id)
);

CREATE INDEX backups_org_idx ON configuration_backups (organization_id, created_at DESC);

CREATE TABLE restore_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    backup_id uuid NOT NULL,
    requested_by uuid NOT NULL REFERENCES users (id),
    status text NOT NULL CHECK (status IN ('accepted', 'rejected', 'pending_reconciliation')),
    detail text NOT NULL DEFAULT '',
    change_request_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (organization_id, backup_id) REFERENCES configuration_backups (organization_id, id)
);

CREATE INDEX restore_runs_org_idx ON restore_runs (organization_id, created_at DESC);

CREATE TABLE integrations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id),
    kind text NOT NULL,
    name text NOT NULL,
    status text NOT NULL,
    detail text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, kind)
);

CREATE TABLE audit_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id),
    actor_user_id uuid,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id uuid,
    request_id text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_events_org_time_idx ON audit_events (organization_id, created_at DESC);
CREATE INDEX audit_events_action_idx ON audit_events (organization_id, action);

CREATE TABLE idempotency_keys (
    organization_id uuid NOT NULL REFERENCES organizations (id),
    key text NOT NULL,
    request_hash text NOT NULL,
    status_code integer NOT NULL,
    response jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, key)
);

CREATE TABLE agent_tasks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    connector_id uuid NOT NULL,
    kind text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'leased', 'succeeded', 'failed')),
    result jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (organization_id, connector_id) REFERENCES connectors (organization_id, id)
);

CREATE INDEX agent_tasks_pending_idx ON agent_tasks (connector_id, status, created_at);

CREATE TABLE simulator_acls (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    device_id uuid NOT NULL,
    policy_version_id uuid NOT NULL,
    spec jsonb NOT NULL,
    applied_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (device_id, policy_version_id),
    FOREIGN KEY (organization_id, device_id) REFERENCES devices (organization_id, id),
    FOREIGN KEY (organization_id, policy_version_id) REFERENCES policy_versions (organization_id, id)
);

CREATE INDEX simulator_acls_device_idx ON simulator_acls (device_id);

CREATE TABLE authority_epochs (
    organization_id uuid NOT NULL REFERENCES organizations (id),
    execution_domain text NOT NULL,
    epoch bigint NOT NULL,
    holder text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, execution_domain)
);

CREATE TABLE worker_heartbeats (
    id text PRIMARY KEY,
    kind text NOT NULL,
    beat_at timestamptz NOT NULL,
    details jsonb NOT NULL DEFAULT '{}'::jsonb
);
