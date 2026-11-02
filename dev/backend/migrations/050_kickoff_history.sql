BEGIN;
CREATE TABLE kickoff_history_import (
 project_id text PRIMARY KEY REFERENCES kickoff_workspace(project_id) ON DELETE CASCADE,
 original_source_id uuid NOT NULL,original_revision_id uuid NOT NULL,
 source_id uuid NOT NULL REFERENCES kickoff_source(id) ON DELETE CASCADE,
 actor text NOT NULL,created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO schema_migration(version) VALUES(50);
COMMIT;
