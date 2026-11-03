BEGIN;
ALTER TABLE kickoff_publication_object DROP CONSTRAINT kickoff_publication_object_kind_check;
ALTER TABLE kickoff_publication_object ADD CONSTRAINT kickoff_publication_object_kind_check
 CHECK(kind IN('device','port','component','connection','placement','facility','location'));
INSERT INTO schema_migration(version) VALUES(51);
COMMIT;
