-- A project remembers the selected existing template reference; this does not approve or execute it.
ALTER TABLE project ADD COLUMN IF NOT EXISTS template_id text NOT NULL DEFAULT '';
INSERT INTO schema_migration(version) VALUES(13) ON CONFLICT DO NOTHING;
