ALTER TABLE bom_import_batch ADD COLUMN IF NOT EXISTS declared_batch_id text;
ALTER TABLE bom_import_batch ADD COLUMN IF NOT EXISTS template_revision_id text NOT NULL DEFAULT 'manual-v1';
CREATE UNIQUE INDEX IF NOT EXISTS declared_bom_batch_unique ON bom_import_batch(project_id,declared_batch_id) WHERE declared_batch_id IS NOT NULL;
INSERT INTO schema_migration(version) VALUES(8) ON CONFLICT DO NOTHING;
