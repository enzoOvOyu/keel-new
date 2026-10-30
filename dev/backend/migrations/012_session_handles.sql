ALTER TABLE api_session ADD COLUMN IF NOT EXISTS session_id uuid NOT NULL DEFAULT gen_random_uuid();
CREATE UNIQUE INDEX IF NOT EXISTS session_handle_unique ON api_session(session_id);
INSERT INTO schema_migration(version) VALUES(12) ON CONFLICT DO NOTHING;
