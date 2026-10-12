-- Saved copies of the reference topology and optional inventory bindings.
-- A binding never owns the inventory device. Removing a symbol does not delete a device.

CREATE TABLE topology_diagrams (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name text NOT NULL,
    mxfile text NOT NULL,
    created_by uuid,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX topology_diagrams_org_idx ON topology_diagrams (organization_id, name, created_at DESC);

CREATE TABLE topology_symbol_bindings (
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    cell_id text NOT NULL,
    device_id uuid,
    updated_by uuid,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, cell_id)
);

CREATE INDEX topology_symbol_bindings_device_idx ON topology_symbol_bindings (organization_id, device_id);

ALTER TABLE topology_diagrams ENABLE ROW LEVEL SECURITY;
ALTER TABLE topology_diagrams FORCE ROW LEVEL SECURITY;
CREATE POLICY topology_diagrams_isolation ON topology_diagrams
    USING (app.org_ok(organization_id)) WITH CHECK (app.org_ok(organization_id));

ALTER TABLE topology_symbol_bindings ENABLE ROW LEVEL SECURITY;
ALTER TABLE topology_symbol_bindings FORCE ROW LEVEL SECURITY;
CREATE POLICY topology_symbol_bindings_isolation ON topology_symbol_bindings
    USING (app.org_ok(organization_id)) WITH CHECK (app.org_ok(organization_id));
