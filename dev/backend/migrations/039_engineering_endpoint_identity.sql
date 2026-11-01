BEGIN;
ALTER TABLE eng_port ADD COLUMN mac text;
ALTER TABLE eng_connection ADD COLUMN termination_type text NOT NULL DEFAULT '';
ALTER TABLE eng_connection ADD COLUMN b_rack_position text NOT NULL DEFAULT '';
CREATE TABLE eng_standard_input_source (
 id uuid PRIMARY KEY,
 revision_id uuid NOT NULL REFERENCES eng_revision(id) ON DELETE CASCADE,
 table_id text NOT NULL,
 contract_version text NOT NULL,
 object_key text NOT NULL,
 content_sha256 text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO schema_migration(version) VALUES(39);
COMMIT;
