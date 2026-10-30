ALTER TABLE cmdb_entity ADD COLUMN catalog_visible boolean NOT NULL DEFAULT true;
CREATE INDEX cmdb_catalog_visible ON cmdb_entity(asset_type,name,item_uid) WHERE catalog_visible;
INSERT INTO schema_migration(version) VALUES(16);
