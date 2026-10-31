-- Search projection only; Kestra remains the execution-state authority.
ALTER TABLE flow_execution_binding ADD COLUMN engine_state text NOT NULL DEFAULT 'UNKNOWN';
ALTER TABLE flow_execution_binding ADD COLUMN state_observed_at timestamptz;
CREATE INDEX flow_execution_state_scope ON flow_execution_binding(project_id,engine_state,created_at DESC,id DESC);
INSERT INTO schema_migration(version) VALUES(22);
