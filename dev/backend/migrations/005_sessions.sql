CREATE TABLE IF NOT EXISTS api_session(token_hash text PRIMARY KEY,csrf_token text NOT NULL,actor text NOT NULL,role text NOT NULL,project_id text NOT NULL,expires_at timestamptz NOT NULL,created_at timestamptz NOT NULL DEFAULT now());
INSERT INTO schema_migration(version) VALUES(5) ON CONFLICT DO NOTHING;
