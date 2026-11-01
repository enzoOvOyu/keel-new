ALTER TABLE eng_source ADD COLUMN base_revision_id uuid REFERENCES eng_revision(id);
ALTER TABLE eng_source ADD COLUMN base_source_id uuid REFERENCES eng_source(id);
-- Difference decisions are review audit and survive project deletion.
CREATE TABLE eng_difference_review (
 id uuid PRIMARY KEY,project_id text NOT NULL,source_id uuid NOT NULL,revision_id uuid NOT NULL,
 base_revision_id uuid,target text NOT NULL,incoming_digest text NOT NULL,base_digest text NOT NULL,
 decision text NOT NULL CHECK(decision IN('accept_incoming','retain_base','retain_current')),
 actor text NOT NULL,reason text NOT NULL,created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX eng_difference_review_lookup ON eng_difference_review(source_id,target,incoming_digest);
INSERT INTO schema_migration(version) VALUES(35);
-- Keep historical planning evidence while removing it from the effective projection.
ALTER TABLE cmdb_field_source ADD COLUMN superseded_at timestamptz;
