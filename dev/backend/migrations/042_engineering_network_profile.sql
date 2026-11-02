BEGIN;
ALTER TABLE eng_network_planes ADD COLUMN profile text NOT NULL DEFAULT '';
INSERT INTO schema_migration(version) VALUES(42);
COMMIT;
