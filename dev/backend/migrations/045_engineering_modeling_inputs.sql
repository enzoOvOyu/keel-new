BEGIN;
CREATE TABLE eng_modeling_input (
 source_id uuid NOT NULL REFERENCES eng_source(id) ON DELETE CASCADE,
 table_id text NOT NULL,
 version integer NOT NULL CHECK(version > 0),
 base_revision integer NOT NULL,
 adopted_revision integer,
 object_key text NOT NULL,
 content_sha256 text NOT NULL,
 row_count integer NOT NULL,
 issues jsonb NOT NULL DEFAULT '[]',
 reason text NOT NULL,
 actor text NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(source_id,table_id)
);
CREATE TABLE eng_kickoff_handoff (
 id uuid PRIMARY KEY,
 source_id uuid NOT NULL REFERENCES eng_source(id) ON DELETE CASCADE,
 project_id text NOT NULL,
 revision integer NOT NULL,
 actor text NOT NULL,
 reason text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(source_id,revision)
);
CREATE TABLE eng_kickoff_handoff_publication (
 handoff_id uuid NOT NULL REFERENCES eng_kickoff_handoff(id) ON DELETE CASCADE,
 target text NOT NULL,
 publication_id uuid NOT NULL REFERENCES eng_publication(id),
 PRIMARY KEY(handoff_id,target)
);
INSERT INTO schema_migration(version) VALUES(45);
COMMIT;
