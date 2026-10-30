ALTER TABLE outbox_event ADD COLUMN IF NOT EXISTS retry_count integer NOT NULL DEFAULT 0;
ALTER TABLE outbox_event ADD COLUMN IF NOT EXISTS next_attempt_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE outbox_event ADD COLUMN IF NOT EXISTS last_error_code text;
ALTER TABLE cmdb_entity ADD COLUMN IF NOT EXISTS revision_id uuid REFERENCES bom_revision(id);
CREATE INDEX IF NOT EXISTS outbox_pending ON outbox_event(next_attempt_at,created_at) WHERE published_at IS NULL;
INSERT INTO schema_migration(version) VALUES(7) ON CONFLICT DO NOTHING;
