BEGIN;
-- Project workspace coordination and versioned input staging. Canonical assets
-- continue to use eng_entity/CMDB; this schema creates no parallel asset identity.
CREATE TABLE kickoff_workspace (
 project_id text PRIMARY KEY REFERENCES project(id) ON DELETE CASCADE,
 revision integer NOT NULL DEFAULT 1 CHECK(revision>0)
);
CREATE TABLE kickoff_input_revision (
 project_id text NOT NULL REFERENCES kickoff_workspace(project_id) ON DELETE CASCADE,
 table_id text NOT NULL,
 version integer NOT NULL CHECK(version>0),
 workspace_revision integer NOT NULL,
 rows jsonb NOT NULL CHECK(jsonb_typeof(rows)='array'),
 source text NOT NULL,actor text NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(project_id,table_id,version)
);
CREATE TABLE kickoff_source (
 id uuid PRIMARY KEY,
 project_id text NOT NULL REFERENCES kickoff_workspace(project_id) ON DELETE CASCADE,
 detached_at timestamptz,
 filename text NOT NULL,object_key text NOT NULL,sha256 text NOT NULL,
 byte_size bigint NOT NULL CHECK(byte_size>=0),
 state text NOT NULL DEFAULT 'awaiting_analysis' CHECK(state IN('awaiting_analysis','recognized','needs_input')),
 created_at timestamptz NOT NULL DEFAULT now(),UNIQUE(project_id,sha256)
);
INSERT INTO schema_migration(version) VALUES(48);
COMMIT;
