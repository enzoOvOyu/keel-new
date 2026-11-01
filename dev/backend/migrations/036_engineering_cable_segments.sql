ALTER TABLE eng_cable_plan ADD COLUMN segment_index integer CHECK(segment_index>0);
INSERT INTO schema_migration(version) VALUES(36);
