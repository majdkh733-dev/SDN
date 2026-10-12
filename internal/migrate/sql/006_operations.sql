-- Operational source of truth for VLAN intent, device configuration versions,
-- connector diagnostics, service accounts, and webhook delivery. Organization
-- disaster-recovery backups stay in configuration_backups.

CREATE TABLE vlans (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id),
    site_id uuid NOT NULL,
    vid integer NOT NULL CHECK (vid BETWEEN 1 AND 4094),
    name text NOT NULL,
    status text NOT NULL DEFAULT 'intended' CHECK (status IN ('intended', 'observed', 'drift')),
    provenance text NOT NULL DEFAULT 'operator',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, site_id, vid),
    FOREIGN KEY (organization_id, site_id) REFERENCES sites (organization_id, id)
);

CREATE INDEX vlans_org_site_idx ON vlans (organization_id, site_id);

CREATE TABLE simulator_vlans (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    device_id uuid NOT NULL,
    policy_version_id uuid NOT NULL,
    vid integer NOT NULL CHECK (vid BETWEEN 1 AND 4094),
    name text NOT NULL,
    interface_name text NOT NULL DEFAULT '',
    UNIQUE (device_id, policy_version_id),
    UNIQUE (organization_id, device_id, vid),
    FOREIGN KEY (organization_id, device_id) REFERENCES devices (organization_id, id)
);

CREATE TABLE simulator_overlays (
    organization_id uuid NOT NULL,
    device_id uuid NOT NULL,
    body text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, device_id),
    FOREIGN KEY (organization_id, device_id) REFERENCES devices (organization_id, id)
);

CREATE TABLE device_config_revisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    device_id uuid NOT NULL,
    role text NOT NULL CHECK (role IN ('desired', 'observed')),
    body text NOT NULL,
    checksum text NOT NULL,
    source text NOT NULL,
    change_request_id uuid,
    collected_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (organization_id, device_id) REFERENCES devices (organization_id, id)
);

CREATE INDEX device_config_device_idx ON device_config_revisions (device_id, collected_at DESC);

CREATE TABLE interface_counters (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    interface_id uuid NOT NULL,
    in_octets bigint NOT NULL CHECK (in_octets >= 0),
    out_octets bigint NOT NULL CHECK (out_octets >= 0),
    in_errors bigint NOT NULL DEFAULT 0,
    out_errors bigint NOT NULL DEFAULT 0,
    sampled_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (organization_id, interface_id) REFERENCES interfaces (organization_id, id)
);

CREATE INDEX interface_counters_idx ON interface_counters (interface_id, sampled_at DESC);

CREATE TABLE service_accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id),
    name text NOT NULL,
    role text NOT NULL,
    token_hash text NOT NULL UNIQUE,
    created_by uuid REFERENCES users (id),
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name)
);

CREATE TABLE webhooks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id),
    name text NOT NULL,
    url text NOT NULL,
    events text[] NOT NULL,
    secret_ref text NOT NULL DEFAULT '',
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name),
    UNIQUE (organization_id, id)
);

CREATE TABLE webhook_deliveries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL,
    webhook_id uuid NOT NULL,
    event text NOT NULL,
    status_code integer NOT NULL DEFAULT 0,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (organization_id, webhook_id) REFERENCES webhooks (organization_id, id)
);

CREATE INDEX webhook_deliveries_idx ON webhook_deliveries (organization_id, created_at DESC);

DO $$
DECLARE tbl text;
BEGIN
    FOREACH tbl IN ARRAY ARRAY[
        'vlans', 'simulator_vlans', 'simulator_overlays', 'device_config_revisions',
        'interface_counters', 'service_accounts', 'webhooks', 'webhook_deliveries'
    ]
    LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', tbl);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', tbl);
        EXECUTE format(
            'CREATE POLICY %I ON %I USING (app.org_ok(organization_id)) WITH CHECK (app.org_ok(organization_id))',
            tbl || '_isolation', tbl
        );
    END LOOP;
END $$;
