-- Cursor for bounded, resumable Kestra discovery. No asset or engine data duplicated.
CREATE TABLE flow_sync_cursor (
  name text PRIMARY KEY,
  window_start timestamptz NOT NULL,
  window_end timestamptz NOT NULL,
  page integer NOT NULL DEFAULT 1 CHECK(page > 0)
);
INSERT INTO schema_migration(version) VALUES(25);
