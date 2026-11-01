BEGIN;
ALTER TABLE eng_port ADD COLUMN connector text NOT NULL DEFAULT '';
INSERT INTO schema_migration(version) VALUES(41);
COMMIT;
