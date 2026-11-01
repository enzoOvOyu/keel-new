BEGIN;
DO $$
DECLARE prior record; updated jsonb; next_id text; relation jsonb := '{"name": "patch_passthrough", "label": "配线节点贯通", "kind": "physical", "source_types": ["port"], "target_types": ["port"], "direction": "outbound", "max_targets": 1, "max_sources": 1, "sources": ["bom", "manual"]}'::jsonb;
BEGIN
 SELECT * INTO prior FROM cmdb_schema_version WHERE status='active' FOR UPDATE;
 IF FOUND AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements(prior.document->'relationships') r WHERE r->>'name'='patch_passthrough') THEN
  next_id:=prior.id||'+engineering-patch-v1';
  updated:=jsonb_set(jsonb_set(prior.document,'{relationships}',(prior.document->'relationships')||jsonb_build_array(relation)),'{version}',to_jsonb(next_id));
  UPDATE cmdb_schema_version SET status='retired' WHERE id=prior.id;
  INSERT INTO cmdb_schema_version(id,document,content_hash,status,created_by) VALUES(next_id,updated,encode(sha256(convert_to(updated::text,'UTF8')),'hex'),'active','migration:030');
  INSERT INTO cmdb_schema_asset_type SELECT next_id,t->>'name',t FROM jsonb_array_elements(updated->'types') t;
  INSERT INTO cmdb_schema_attribute SELECT next_id,t->>'name',a->>'name',a FROM jsonb_array_elements(updated->'types') t,jsonb_array_elements(t->'attributes') a;
  INSERT INTO cmdb_relationship_definition SELECT next_id,r->>'name',r FROM jsonb_array_elements(updated->'relationships') r;
 END IF;
END $$;
INSERT INTO schema_migration(version) VALUES(30);
COMMIT;
