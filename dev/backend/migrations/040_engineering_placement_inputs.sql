BEGIN;
ALTER TABLE eng_equipment ADD COLUMN height_u integer CHECK(height_u>0);
ALTER TABLE eng_facility ADD COLUMN code text NOT NULL DEFAULT '';
ALTER TABLE eng_placement ADD COLUMN installation_type text NOT NULL DEFAULT '';
ALTER TABLE eng_placement ADD COLUMN position_code text NOT NULL DEFAULT '';
INSERT INTO schema_migration(version) VALUES(40);
COMMIT;
