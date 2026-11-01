-- Immutable source references and typed physical revisions; no live data conversion.
CREATE TABLE eng_source (
 id uuid PRIMARY KEY, project_id text NOT NULL REFERENCES project(id) ON DELETE CASCADE,
 import_id uuid NOT NULL UNIQUE REFERENCES bom_import_batch(id) ON DELETE CASCADE,
 revision integer NOT NULL DEFAULT 1 CHECK(revision>0), head_revision_id uuid,
 summary text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(id,project_id)
);
CREATE TABLE eng_revision (
 id uuid PRIMARY KEY, source_id uuid NOT NULL REFERENCES eng_source(id) ON DELETE CASCADE,
 sequence integer NOT NULL CHECK(sequence>0), actor text NOT NULL, origin text NOT NULL CHECK(origin IN('agent','human')),
 reason text NOT NULL, total_cells integer NOT NULL, mapped_cells integer NOT NULL,
 proposal jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(source_id,sequence),UNIQUE(id,source_id)
);
ALTER TABLE eng_source ADD CONSTRAINT eng_head_fk FOREIGN KEY(head_revision_id,id) REFERENCES eng_revision(id,source_id) DEFERRABLE INITIALLY DEFERRED;
CREATE TABLE eng_entity (
 id uuid NOT NULL, project_id text NOT NULL REFERENCES project(id) ON DELETE CASCADE,
 kind text NOT NULL, planning_key text NOT NULL, bom_item_uid uuid REFERENCES bom_item(item_uid),
 PRIMARY KEY(project_id,id),UNIQUE(project_id,kind,planning_key)
);
CREATE TABLE eng_entity_version (
 revision_id uuid NOT NULL REFERENCES eng_revision(id) ON DELETE CASCADE,entity_id uuid NOT NULL,
 project_id text NOT NULL,kind text NOT NULL,maturity text NOT NULL CHECK(maturity IN('Estimated','Calculated','Confirmed','As-built')),
 provenance_id uuid NOT NULL, PRIMARY KEY(revision_id,entity_id),
 FOREIGN KEY(project_id,entity_id) REFERENCES eng_entity(project_id,id) ON DELETE CASCADE
);
CREATE TABLE eng_equipment (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,equipment_type text NOT NULL,model_ref text,
 source_model_label text NOT NULL,quantity bigint NOT NULL,identity_kind text NOT NULL CHECK(identity_kind IN('group','planned_instance','observed_reference')),
 label text NOT NULL,serial text,PRIMARY KEY(revision_id,entity_id),
 FOREIGN KEY(revision_id,entity_id) REFERENCES eng_entity_version(revision_id,entity_id) ON DELETE CASCADE
);
CREATE TABLE eng_assembly (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,parent_id uuid NOT NULL,child_id uuid NOT NULL,
 relation_type text NOT NULL,quantity bigint NOT NULL,slot_label text,PRIMARY KEY(revision_id,entity_id),
 FOREIGN KEY(revision_id,entity_id) REFERENCES eng_entity_version(revision_id,entity_id) ON DELETE CASCADE
);
CREATE TABLE eng_facility (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,kind text NOT NULL CHECK(kind IN('site','room','rack')),label text NOT NULL,parent_id uuid,
 capacity_u integer,width_mm numeric,height_mm numeric,depth_mm numeric,x_mm numeric,y_mm numeric,z_mm numeric,orientation_degrees numeric,coordinate_frame_id uuid,
 PRIMARY KEY(revision_id,entity_id),FOREIGN KEY(revision_id,entity_id) REFERENCES eng_entity_version(revision_id,entity_id) ON DELETE CASCADE
);
CREATE TABLE eng_placement (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,equipment_id uuid NOT NULL,rack_id uuid NOT NULL,
 start_u integer,height_u integer,mount_face text NOT NULL,depth_mm numeric,PRIMARY KEY(revision_id,entity_id),
 FOREIGN KEY(revision_id,entity_id) REFERENCES eng_entity_version(revision_id,entity_id) ON DELETE CASCADE
);
-- Evidence is immutable and per entity/field, and is never an AI-generated value.
CREATE TABLE eng_evidence (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,id uuid NOT NULL,sheet text NOT NULL,cell text NOT NULL,field text NOT NULL,formula boolean NOT NULL,
 PRIMARY KEY(revision_id,entity_id,id),FOREIGN KEY(revision_id,entity_id) REFERENCES eng_entity_version(revision_id,entity_id) ON DELETE CASCADE
);
CREATE TABLE eng_issue (
 revision_id uuid NOT NULL REFERENCES eng_revision(id) ON DELETE CASCADE,id uuid NOT NULL,entity_id uuid NOT NULL,
 code text NOT NULL,severity text NOT NULL,module_id text NOT NULL,field text NOT NULL,message text NOT NULL,target text NOT NULL,
 PRIMARY KEY(revision_id,id)
);
CREATE TABLE eng_target (
 source_id uuid NOT NULL REFERENCES eng_source(id) ON DELETE CASCADE,target text NOT NULL CHECK(target IN('CMDB_PLANNED_EQUIPMENT','CMDB_PLANNED_FACILITY','CMDB_PLANNED_PLACEMENT')),
 revision_id uuid NOT NULL,digest text NOT NULL,state text NOT NULL CHECK(state IN('DETECTED','NEEDS_INPUT','READY','PROCESSING','IN_REVIEW','APPROVED','PUBLISHED','BLOCKED','STALE','NOT_APPLICABLE')),
 review_id uuid,publication_id uuid,PRIMARY KEY(source_id,target),
 FOREIGN KEY(revision_id,source_id) REFERENCES eng_revision(id,source_id) ON DELETE CASCADE
);
-- Review/publication audit survives project removal; source/revision IDs are historical references.
CREATE TABLE eng_review (
 id uuid PRIMARY KEY,project_id text NOT NULL,source_id uuid NOT NULL,target text NOT NULL,revision_id uuid NOT NULL,
 digest text NOT NULL,decision text NOT NULL CHECK(decision IN('approve','reject')),actor text NOT NULL,reason text NOT NULL,created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE eng_publication (
 id uuid PRIMARY KEY,project_id text NOT NULL,source_id uuid NOT NULL,target text NOT NULL,revision_id uuid NOT NULL,
 review_id uuid NOT NULL REFERENCES eng_review(id),digest text NOT NULL,idempotency_key text NOT NULL,outbox_id uuid NOT NULL,
 state text NOT NULL CHECK(state IN('pending','published','failed')),error_code text,receipt text,created_at timestamptz NOT NULL DEFAULT now(),published_at timestamptz,
 UNIQUE(project_id,idempotency_key,target)
);
CREATE TABLE eng_job (
 id uuid PRIMARY KEY,source_id uuid NOT NULL REFERENCES eng_source(id) ON DELETE CASCADE,project_id text NOT NULL REFERENCES project(id) ON DELETE CASCADE,
 expected_revision integer NOT NULL,state text NOT NULL CHECK(state IN('pending','running','succeeded','failed')),
 error_code text,claim_id uuid,proposal jsonb,created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX eng_one_active_job ON eng_job(source_id) WHERE state IN('pending','running');
CREATE TABLE eng_step (
 job_id uuid NOT NULL REFERENCES eng_job(id) ON DELETE CASCADE,sequence integer NOT NULL,tool text NOT NULL,detail text NOT NULL,state text NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(job_id,sequence)
);
DO $$
DECLARE prior record; updated jsonb; next_id text;
BEGIN
 SELECT * INTO prior FROM cmdb_schema_version WHERE status='active' FOR UPDATE;
 IF FOUND THEN
  next_id := prior.id || '+engineering-physical-v1';
  SELECT jsonb_set(prior.document,'{relationships}',jsonb_agg(CASE WHEN r->>'name'='contains' AND NOT (r->'target_types' ? 'device') THEN jsonb_set(r,'{target_types}',(r->'target_types')||'"device"'::jsonb) ELSE r END ORDER BY ord)) INTO updated
  FROM jsonb_array_elements(prior.document->'relationships') WITH ORDINALITY AS x(r,ord);
  updated := jsonb_set(updated,'{version}',to_jsonb(next_id));
  UPDATE cmdb_schema_version SET status='retired' WHERE id=prior.id;
  INSERT INTO cmdb_schema_version(id,document,content_hash,status,created_by) VALUES(next_id,updated,encode(sha256(convert_to(updated::text,'UTF8')),'hex'),'active','migration:027');
  INSERT INTO cmdb_schema_asset_type SELECT next_id,name,definition FROM cmdb_schema_asset_type WHERE schema_version=prior.id;
  INSERT INTO cmdb_schema_attribute SELECT next_id,asset_type,field_name,definition FROM cmdb_schema_attribute WHERE schema_version=prior.id;
  INSERT INTO cmdb_relationship_definition SELECT next_id,r->>'name',r FROM jsonb_array_elements(updated->'relationships') r;
 END IF;
END $$;
INSERT INTO schema_migration(version) VALUES(27);
