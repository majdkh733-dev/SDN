ALTER TABLE connectors DROP CONSTRAINT IF EXISTS connectors_adapter_check;
ALTER TABLE connectors
    ADD CONSTRAINT connectors_adapter_check
    CHECK (adapter IN ('simulator', 'ovn', 'netconf', 'gnmi', 'vendor_api', 'site'));
