-- Site connectors, bounded discovery, asset identity, and per-organization
-- navigation and theme preferences. Connector credentials stay hashed.
-- Device health is independent of connector reachability.

ALTER TABLE sites
    ADD COLUMN latitude double precision,
    ADD COLUMN longitude double precision;

ALTER TABLE connectors
    ADD COLUMN description text NOT NULL DEFAULT '',
    ADD COLUMN deployment_method text NOT NULL DEFAULT '',
    ADD COLUMN status text NOT NULL DEFAULT 'not_enrolled',
    ADD COLUMN host_info jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN capabilities jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN enrolled_at timestamptz,
    ADD COLUMN revoked_at timestamptz;

ALTER TABLE connectors DROP CONSTRAINT IF EXISTS connectors_adapter_check;
ALTER TABLE connectors
    ADD CONSTRAINT connectors_adapter_check
    CHECK (adapter IN ('simulator', 'ovn', 'netconf', 'gnmi', 'vendor_api', 'site'));

ALTER TABLE connectors
    ADD CONSTRAINT connectors_status_check
    CHECK (status IN ('not_enrolled', 'connecting', 'online', 'degraded', 'offline', 'revoked'));

UPDATE connectors SET status = CASE health
    WHEN 'healthy' THEN 'online'
    WHEN 'degraded' THEN 'degraded'
    WHEN 'disconnected' THEN 'offline'
    ELSE 'not_enrolled'
END;

ALTER TABLE connector_tokens
    ADD COLUMN last_used_at timestamptz;

CREATE TABLE enrollment_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    connector_id uuid NOT NULL,
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, connector_id) REFERENCES connectors (organization_id, id)
);

CREATE INDEX enrollment_tokens_connector_idx
    ON enrollment_tokens (organization_id, connector_id, created_at DESC);

ALTER TABLE devices DROP CONSTRAINT IF EXISTS devices_data_origin_check;
ALTER TABLE devices
    ADD CONSTRAINT devices_data_origin_check
    CHECK (data_origin IN ('simulator', 'ovn', 'seed', 'discovery', 'csv'));

ALTER TABLE devices
    ADD COLUMN hostname text NOT NULL DEFAULT '',
    ADD COLUMN mac text NOT NULL DEFAULT '',
    ADD COLUMN vendor text NOT NULL DEFAULT '',
    ADD COLUMN serial text NOT NULL DEFAULT '',
    ADD COLUMN identity_key text NOT NULL DEFAULT '',
    ADD COLUMN first_seen_at timestamptz,
    ADD COLUMN last_seen_at timestamptz,
    ADD COLUMN discovery_source text NOT NULL DEFAULT '',
    ADD COLUMN locked_fields text[] NOT NULL DEFAULT '{}';

CREATE UNIQUE INDEX devices_identity_idx
    ON devices (organization_id, identity_key)
    WHERE identity_key <> '';

CREATE INDEX devices_site_ip_idx
    ON devices (organization_id, site_id, management_endpoint);

CREATE TABLE discovery_profiles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    site_id uuid NOT NULL,
    connector_id uuid NOT NULL,
    name text NOT NULL,
    targets jsonb NOT NULL,
    exclusions jsonb NOT NULL DEFAULT '[]'::jsonb,
    scan_profile text NOT NULL DEFAULT 'reachability',
    timeout_ms integer NOT NULL DEFAULT 800 CHECK (timeout_ms >= 100 AND timeout_ms <= 5000),
    concurrency integer NOT NULL DEFAULT 8 CHECK (concurrency >= 1 AND concurrency <= 32),
    schedule_minutes integer NOT NULL DEFAULT 0 CHECK (schedule_minutes >= 0 AND schedule_minutes <= 10080),
    auto_import boolean NOT NULL DEFAULT false,
    credential_ref text NOT NULL DEFAULT '',
    next_run_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, id),
    UNIQUE (organization_id, site_id, name),
    FOREIGN KEY (organization_id, site_id) REFERENCES sites (organization_id, id),
    FOREIGN KEY (organization_id, connector_id) REFERENCES connectors (organization_id, id)
);

CREATE INDEX discovery_profiles_due_idx
    ON discovery_profiles (next_run_at)
    WHERE schedule_minutes > 0;

CREATE TABLE discovery_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    site_id uuid NOT NULL,
    connector_id uuid NOT NULL,
    profile_id uuid,
    status text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'completed', 'partial', 'failed', 'cancelled')),
    progress integer NOT NULL DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
    discovered_count integer NOT NULL DEFAULT 0,
    error text NOT NULL DEFAULT '',
    lease_until timestamptz,
    attempt integer NOT NULL DEFAULT 0,
    targets jsonb NOT NULL DEFAULT '[]'::jsonb,
    started_at timestamptz,
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, connector_id) REFERENCES connectors (organization_id, id),
    FOREIGN KEY (organization_id, site_id) REFERENCES sites (organization_id, id),
    FOREIGN KEY (organization_id, profile_id) REFERENCES discovery_profiles (organization_id, id)
);

CREATE INDEX discovery_jobs_lease_idx
    ON discovery_jobs (connector_id, status, created_at);

CREATE TABLE discovery_observations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    job_id uuid NOT NULL,
    site_id uuid NOT NULL,
    connector_id uuid NOT NULL,
    ip text NOT NULL DEFAULT '',
    hostname text NOT NULL DEFAULT '',
    mac text NOT NULL DEFAULT '',
    vendor text NOT NULL DEFAULT '',
    model text NOT NULL DEFAULT '',
    device_type text NOT NULL DEFAULT '',
    serial text NOT NULL DEFAULT '',
    os_version text NOT NULL DEFAULT '',
    reachable boolean NOT NULL DEFAULT false,
    methods jsonb NOT NULL DEFAULT '[]'::jsonb,
    confidence text NOT NULL DEFAULT 'low',
    identity_key text NOT NULL DEFAULT '',
    matched_device_id uuid,
    match_status text NOT NULL DEFAULT 'new' CHECK (match_status IN ('new', 'matched', 'ambiguous')),
    imported boolean NOT NULL DEFAULT false,
    UNIQUE (organization_id, id),
    UNIQUE (job_id, identity_key),
    FOREIGN KEY (organization_id, job_id) REFERENCES discovery_jobs (organization_id, id)
);

CREATE INDEX discovery_observations_job_idx ON discovery_observations (job_id);

CREATE TABLE asset_imports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    device_id uuid,
    job_id uuid,
    observation_id uuid,
    action text NOT NULL,
    detail text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX asset_imports_device_idx ON asset_imports (organization_id, device_id, created_at DESC);

CREATE TABLE site_links (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    a_site_id uuid NOT NULL,
    b_site_id uuid NOT NULL,
    source text NOT NULL DEFAULT 'manual',
    status text NOT NULL DEFAULT 'unknown',
    latency_ms double precision,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (a_site_id <> b_site_id),
    UNIQUE (organization_id, a_site_id, b_site_id),
    FOREIGN KEY (organization_id, a_site_id) REFERENCES sites (organization_id, id),
    FOREIGN KEY (organization_id, b_site_id) REFERENCES sites (organization_id, id)
);

CREATE TABLE org_preferences (
    organization_id uuid PRIMARY KEY REFERENCES organizations (id),
    navigation jsonb NOT NULL DEFAULT '{}'::jsonb,
    theme jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE enrollment_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment_tokens FORCE ROW LEVEL SECURITY;
CREATE POLICY enrollment_tokens_isolation ON enrollment_tokens
    USING (app.org_ok(organization_id)) WITH CHECK (app.org_ok(organization_id));

ALTER TABLE discovery_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE discovery_profiles FORCE ROW LEVEL SECURITY;
CREATE POLICY discovery_profiles_isolation ON discovery_profiles
    USING (app.org_ok(organization_id)) WITH CHECK (app.org_ok(organization_id));

ALTER TABLE discovery_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE discovery_jobs FORCE ROW LEVEL SECURITY;
CREATE POLICY discovery_jobs_isolation ON discovery_jobs
    USING (app.org_ok(organization_id)) WITH CHECK (app.org_ok(organization_id));

ALTER TABLE discovery_observations ENABLE ROW LEVEL SECURITY;
ALTER TABLE discovery_observations FORCE ROW LEVEL SECURITY;
CREATE POLICY discovery_observations_isolation ON discovery_observations
    USING (app.org_ok(organization_id)) WITH CHECK (app.org_ok(organization_id));

ALTER TABLE asset_imports ENABLE ROW LEVEL SECURITY;
ALTER TABLE asset_imports FORCE ROW LEVEL SECURITY;
CREATE POLICY asset_imports_isolation ON asset_imports
    USING (app.org_ok(organization_id)) WITH CHECK (app.org_ok(organization_id));

ALTER TABLE site_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE site_links FORCE ROW LEVEL SECURITY;
CREATE POLICY site_links_isolation ON site_links
    USING (app.org_ok(organization_id)) WITH CHECK (app.org_ok(organization_id));

ALTER TABLE org_preferences ENABLE ROW LEVEL SECURITY;
ALTER TABLE org_preferences FORCE ROW LEVEL SECURITY;
CREATE POLICY org_preferences_isolation ON org_preferences
    USING (app.org_ok(organization_id)) WITH CHECK (app.org_ok(organization_id));
