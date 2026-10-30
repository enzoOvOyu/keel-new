ALTER TABLE bom_import_batch ADD COLUMN IF NOT EXISTS review_model_bindings jsonb NOT NULL DEFAULT '{}';
CREATE TABLE IF NOT EXISTS bom_review(id uuid PRIMARY KEY,import_id uuid NOT NULL REFERENCES bom_import_batch(id),revision integer NOT NULL,actor text NOT NULL,model_bindings jsonb NOT NULL,reason text NOT NULL,created_at timestamptz NOT NULL DEFAULT now());
INSERT INTO schema_migration(version) VALUES(9) ON CONFLICT DO NOTHING;
