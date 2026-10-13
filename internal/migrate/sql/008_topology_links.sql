-- Manual association of a reference line with inventory interfaces.
-- Clearing a mapping does not delete interfaces or devices.

CREATE TABLE topology_link_bindings (
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    cell_id text NOT NULL,
    source_device_id uuid,
    source_interface_id uuid,
    target_device_id uuid,
    target_interface_id uuid,
    connection_type text NOT NULL DEFAULT 'documented',
    evidence text NOT NULL DEFAULT 'manual',
    updated_by uuid,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, cell_id)
);

ALTER TABLE topology_link_bindings ENABLE ROW LEVEL SECURITY;
ALTER TABLE topology_link_bindings FORCE ROW LEVEL SECURITY;
CREATE POLICY topology_link_bindings_isolation ON topology_link_bindings
    USING (app.org_ok(organization_id)) WITH CHECK (app.org_ok(organization_id));
