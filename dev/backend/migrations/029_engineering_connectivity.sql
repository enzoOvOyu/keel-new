BEGIN;
ALTER TABLE eng_target DROP CONSTRAINT eng_target_target_check;
ALTER TABLE eng_target ADD CONSTRAINT eng_target_target_check CHECK(target IN('CMDB_PLANNED_EQUIPMENT','CMDB_PLANNED_FACILITY','CMDB_PLANNED_PLACEMENT','CMDB_PLANNED_CONNECTIVITY','ENGINEERING_CABLING'));
CREATE TABLE eng_port (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,equipment_id uuid NOT NULL,name text NOT NULL,
 parent_port_id uuid,breakout_index integer,role text NOT NULL,rate_bps bigint,medium text NOT NULL,
 PRIMARY KEY(revision_id,entity_id),FOREIGN KEY(revision_id,entity_id) REFERENCES eng_entity_version(revision_id,entity_id) ON DELETE CASCADE
);
CREATE TABLE eng_connection (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,a_port_id uuid NOT NULL,z_port_id uuid NOT NULL,patch_port_ids uuid[] NOT NULL,
 PRIMARY KEY(revision_id,entity_id),FOREIGN KEY(revision_id,entity_id) REFERENCES eng_entity_version(revision_id,entity_id) ON DELETE CASCADE
);
CREATE TABLE eng_cable_plan (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,connection_id uuid NOT NULL,label text,
 spec_id uuid NOT NULL,medium text NOT NULL,connector_a text NOT NULL,connector_z text NOT NULL,rate_bps bigint,source_model_label text NOT NULL,
 rule_id uuid,rule_version integer,
 PRIMARY KEY(revision_id,entity_id),FOREIGN KEY(revision_id,entity_id) REFERENCES eng_entity_version(revision_id,entity_id) ON DELETE CASCADE
);
CREATE TABLE eng_cable_route (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,position integer NOT NULL,from_id uuid NOT NULL,to_id uuid NOT NULL,
 distance_value numeric,distance_unit text,calculation_run_id uuid REFERENCES eng_calculation_run(id),
 PRIMARY KEY(revision_id,entity_id,position),FOREIGN KEY(revision_id,entity_id) REFERENCES eng_cable_plan(revision_id,entity_id) ON DELETE CASCADE
);
CREATE TABLE eng_cable_length (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,kind text NOT NULL CHECK(kind IN('estimated','calculated','selected_stock','confirmed','as_built')),
 value numeric NOT NULL,unit text NOT NULL,evidence_ids uuid[] NOT NULL,calculation_run_id uuid REFERENCES eng_calculation_run(id),
 PRIMARY KEY(revision_id,entity_id,kind),FOREIGN KEY(revision_id,entity_id) REFERENCES eng_cable_plan(revision_id,entity_id) ON DELETE CASCADE
);
CREATE TABLE eng_optic_demand (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,physical_port_id uuid NOT NULL,source_model_label text NOT NULL,quantity bigint NOT NULL,
 PRIMARY KEY(revision_id,entity_id),FOREIGN KEY(revision_id,entity_id) REFERENCES eng_entity_version(revision_id,entity_id) ON DELETE CASCADE
);
DO $$
DECLARE prior record; updated jsonb; next_id text; additions jsonb := '{"port": [{"name": "parent_port_id", "label": "父物理端口", "type": "Reference", "required": false, "unique": false, "editable": false, "sources": ["bom", "agent", "bmc", "switch", "derived"], "models": [], "group": "基础属性", "rules": {}}, {"name": "breakout_index", "label": "Breakout序号", "type": "Number", "required": false, "unique": false, "editable": false, "sources": ["bom", "agent", "bmc", "switch", "derived"], "models": [], "group": "基础属性", "rules": {}}, {"name": "rate_bps", "label": "规划速率 bit/s", "type": "Number", "required": false, "unique": false, "editable": false, "sources": ["bom", "agent", "bmc", "switch", "derived"], "models": [], "group": "基础属性", "rules": {}}], "cable": [{"name": "connection_plan_id", "label": "连接规划", "type": "Reference", "required": false, "unique": false, "editable": false, "sources": ["bom", "agent", "bmc", "switch", "derived"], "models": [], "group": "基础属性", "rules": {}}, {"name": "segment_index", "label": "路径段序号", "type": "Number", "required": false, "unique": false, "editable": false, "sources": ["bom", "agent", "bmc", "switch", "derived"], "models": [], "group": "基础属性", "rules": {}}]}'::jsonb;
BEGIN
 SELECT * INTO prior FROM cmdb_schema_version WHERE status='active' FOR UPDATE;
 IF FOUND THEN
  next_id:=prior.id||'+engineering-connectivity-v1';
  SELECT jsonb_set(prior.document,'{types}',jsonb_agg(CASE WHEN additions ? (t->>'name') THEN jsonb_set(t,'{attributes}',(t->'attributes')||COALESCE((SELECT jsonb_agg(a) FROM jsonb_array_elements(additions->(t->>'name')) a WHERE NOT EXISTS(SELECT 1 FROM jsonb_array_elements(t->'attributes') old WHERE old->>'name'=a->>'name')),'[]'::jsonb)) ELSE t END ORDER BY ord)) INTO updated FROM jsonb_array_elements(prior.document->'types') WITH ORDINALITY x(t,ord);
  updated:=jsonb_set(updated,'{version}',to_jsonb(next_id));
  UPDATE cmdb_schema_version SET status='retired' WHERE id=prior.id;
  INSERT INTO cmdb_schema_version(id,document,content_hash,status,created_by) VALUES(next_id,updated,encode(sha256(convert_to(updated::text,'UTF8')),'hex'),'active','migration:029');
  INSERT INTO cmdb_schema_asset_type SELECT next_id,t->>'name',t FROM jsonb_array_elements(updated->'types') t;
  INSERT INTO cmdb_schema_attribute SELECT next_id,t->>'name',a->>'name',a FROM jsonb_array_elements(updated->'types') t,jsonb_array_elements(t->'attributes') a;
  INSERT INTO cmdb_relationship_definition SELECT next_id,r->>'name',r FROM jsonb_array_elements(updated->'relationships') r;
 END IF;
END $$;
INSERT INTO schema_migration(version) VALUES(29);
COMMIT;
