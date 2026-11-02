BEGIN;
ALTER TABLE kickoff_workspace ADD COLUMN source_id uuid REFERENCES eng_source(id) ON DELETE SET NULL;
CREATE TABLE kickoff_calculation (
 id uuid PRIMARY KEY,project_id text NOT NULL REFERENCES kickoff_workspace(project_id) ON DELETE CASCADE,
 domain text NOT NULL CHECK(domain IN('cabling','power')),workspace_revision integer NOT NULL,
 input_digest text NOT NULL,status text NOT NULL CHECK(status IN('ready','partial','needs_input')),
 rows jsonb NOT NULL,power jsonb NOT NULL,issues jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE kickoff_issue_decision (
 project_id text NOT NULL REFERENCES kickoff_workspace(project_id) ON DELETE CASCADE,
 issue_id uuid NOT NULL,workspace_revision integer NOT NULL,actor text NOT NULL,value text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(project_id,issue_id,workspace_revision)
);
CREATE TABLE kickoff_publication (
 id uuid PRIMARY KEY,project_id text NOT NULL REFERENCES kickoff_workspace(project_id) ON DELETE CASCADE,
 revision integer NOT NULL,expected_revision integer NOT NULL,model_revision uuid NOT NULL REFERENCES eng_revision(id) DEFERRABLE INITIALLY DEFERRED,
 actor text NOT NULL,reason text NOT NULL,import_id uuid NOT NULL REFERENCES bom_import_batch(id) DEFERRABLE INITIALLY DEFERRED,
 state text NOT NULL DEFAULT 'pending' CHECK(state IN('pending','published','failed')),error_code text,
 outbox_id uuid NOT NULL REFERENCES outbox_event(id) DEFERRABLE INITIALLY DEFERRED,created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(project_id,revision),UNIQUE(project_id,expected_revision)
);
CREATE TABLE kickoff_publication_object (
 publication_id uuid NOT NULL REFERENCES kickoff_publication(id) ON DELETE CASCADE,
 entity_id uuid NOT NULL,kind text NOT NULL CHECK(kind IN('device','port','component','connection','placement')),
 PRIMARY KEY(publication_id,entity_id,kind)
);
CREATE TABLE kickoff_publication_input (
 publication_id uuid NOT NULL REFERENCES kickoff_publication(id) ON DELETE CASCADE,
 project_id text NOT NULL,table_id text NOT NULL,version integer NOT NULL,
 PRIMARY KEY(publication_id,table_id),
 FOREIGN KEY(project_id,table_id,version) REFERENCES kickoff_input_revision(project_id,table_id,version) ON DELETE CASCADE
);
INSERT INTO schema_migration(version) VALUES(49);
COMMIT;
