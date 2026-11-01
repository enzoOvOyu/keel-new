-- Engineering uploads parse once; only the engineering Agent interprets them.
ALTER TABLE bom_import_batch DROP CONSTRAINT bom_import_batch_import_mode_check;
ALTER TABLE bom_import_batch ADD CONSTRAINT bom_import_batch_import_mode_check CHECK(import_mode IN ('legacy','ai','fixed','engineering'));

-- Large tool results/checkpoints live in the project's MinIO prefix. Core facts
-- and engineering models remain in their existing typed relational tables.
CREATE TABLE eng_analysis_artifact (
 id uuid PRIMARY KEY,
 job_id uuid NOT NULL REFERENCES eng_job(id) ON DELETE CASCADE,
 kind text NOT NULL,
 state text NOT NULL,
 object_key text NOT NULL UNIQUE,
 content_sha256 text NOT NULL,
 byte_size bigint NOT NULL CHECK(byte_size>=0),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX eng_analysis_artifact_job ON eng_analysis_artifact(job_id,created_at DESC);
INSERT INTO schema_migration(version) VALUES(38);
