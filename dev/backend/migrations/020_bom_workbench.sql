ALTER TABLE bom_import_batch ADD COLUMN import_mode text NOT NULL DEFAULT 'legacy' CHECK(import_mode IN ('legacy','ai','fixed'));
ALTER TABLE bom_import_batch ADD COLUMN workbench jsonb NOT NULL DEFAULT '{}';
ALTER TABLE bom_import_batch ADD COLUMN provider_id uuid REFERENCES ai_provider_config(id);
ALTER TABLE bom_revision DROP CONSTRAINT bom_revision_import_id_key;
CREATE TABLE bom_submission(id uuid PRIMARY KEY,import_id uuid NOT NULL REFERENCES bom_import_batch(id), project_id text NOT NULL, idempotency_key text NOT NULL, request_digest text NOT NULL, revision_id uuid NOT NULL REFERENCES bom_revision(id), results jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(project_id,idempotency_key));
ALTER TABLE bom_job ADD COLUMN parameters jsonb NOT NULL DEFAULT '{}';
INSERT INTO schema_migration(version) VALUES(20);
