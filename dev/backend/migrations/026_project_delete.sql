-- Prevent a previously authorized upload from recreating a deleted project scope.
ALTER TABLE bom_import_batch ADD CONSTRAINT bom_import_project_exists FOREIGN KEY(project_id) REFERENCES project(id) NOT VALID;
-- Approval decisions and Flow audit are historical evidence, not live scopes.
ALTER TABLE approval_record DROP CONSTRAINT approval_record_project_id_fkey;
ALTER TABLE flow_audit DROP CONSTRAINT flow_audit_project_id_fkey;
INSERT INTO schema_migration(version) VALUES(26) ON CONFLICT DO NOTHING;
