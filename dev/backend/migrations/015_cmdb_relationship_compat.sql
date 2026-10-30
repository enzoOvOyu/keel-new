-- Map historical BOM edge semantics into the typed relationship definitions.
UPDATE cmdb_relationship SET from_uid=to_uid,to_uid=from_uid,definition_id='contains',kind='composition',cardinality='one-to-many' WHERE kind='contained_by';
UPDATE cmdb_relationship SET definition_id=kind,kind='physical',cardinality='many-to-one' WHERE kind IN('cable_endpoint_a','cable_endpoint_b');
UPDATE cmdb_relationship SET definition_id='located_in',kind='position',cardinality='many-to-one' WHERE kind='located_in';
INSERT INTO schema_migration(version) VALUES(15);
