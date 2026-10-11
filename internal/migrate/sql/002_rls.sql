-- Row-level security is defense in depth. Every tenant-owned query also filters
-- organization_id in application SQL. app.bypass is reserved for trusted
-- maintenance (schema seed and the workflow claim) and is never set by request
-- handlers. app.current_org and app.current_user are transaction-local.

CREATE SCHEMA IF NOT EXISTS app;

CREATE OR REPLACE FUNCTION app.bypass_ok() RETURNS boolean
LANGUAGE sql STABLE AS $$
    SELECT current_setting('app.bypass', true) = 'on'
$$;

CREATE OR REPLACE FUNCTION app.org_ok(org uuid) RETURNS boolean
LANGUAGE sql STABLE AS $$
    SELECT app.bypass_ok() OR org::text = current_setting('app.current_org', true)
$$;

CREATE OR REPLACE FUNCTION app.user_ok(uid uuid) RETURNS boolean
LANGUAGE sql STABLE AS $$
    SELECT uid::text = current_setting('app.current_user', true)
$$;

ALTER TABLE organizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE organizations FORCE ROW LEVEL SECURITY;
CREATE POLICY organizations_isolation ON organizations
    USING (
        app.bypass_ok()
        OR id::text = current_setting('app.current_org', true)
        OR EXISTS (
            SELECT 1 FROM memberships m
            WHERE m.organization_id = organizations.id
              AND app.user_ok(m.user_id)
        )
    )
    WITH CHECK (app.bypass_ok() OR id::text = current_setting('app.current_org', true));

ALTER TABLE memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE memberships FORCE ROW LEVEL SECURITY;
CREATE POLICY memberships_isolation ON memberships
    USING (app.org_ok(organization_id) OR app.user_ok(user_id))
    WITH CHECK (app.bypass_ok() OR organization_id::text = current_setting('app.current_org', true));

DO $$
DECLARE
    tbl text;
BEGIN
    FOREACH tbl IN ARRAY ARRAY[
        'sites', 'connectors', 'connector_tokens', 'devices', 'interfaces',
        'topology_links', 'capability_snapshots', 'fabrics', 'tenants', 'segments',
        'policies', 'policy_versions', 'desired_states', 'observed_states',
        'drift_findings', 'change_requests', 'approvals', 'workflow_runs',
        'execution_steps', 'verification_results', 'configuration_backups',
        'restore_runs', 'integrations', 'audit_events', 'idempotency_keys',
        'agent_tasks', 'simulator_acls', 'authority_epochs'
    ]
    LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', tbl);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', tbl);
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I', tbl || '_isolation', tbl);
        EXECUTE format(
            'CREATE POLICY %I ON %I USING (app.org_ok(organization_id)) WITH CHECK (app.org_ok(organization_id))',
            tbl || '_isolation', tbl
        );
    END LOOP;
END $$;
