-- A hazardous approval authorizes one frozen job, not an unlimited execution token.
ALTER TABLE flow_execution_binding ADD COLUMN approval_id uuid REFERENCES approval_record(id);
CREATE UNIQUE INDEX flow_execution_approval_once ON flow_execution_binding(approval_id) WHERE approval_id IS NOT NULL;
INSERT INTO schema_migration(version) VALUES(24);
