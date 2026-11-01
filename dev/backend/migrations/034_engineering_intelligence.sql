-- Immutable import plans, module bindings, regions and proposed semantic facts.
CREATE TABLE eng_provenance (
 id uuid PRIMARY KEY REFERENCES eng_revision(id) ON DELETE CASCADE,producer_type text NOT NULL,producer_id text NOT NULL,
 producer_version text NOT NULL,run_id uuid NOT NULL,actor_id text,created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE eng_provenance_input (
 provenance_id uuid NOT NULL REFERENCES eng_provenance(id) ON DELETE CASCADE,kind text NOT NULL,id uuid NOT NULL,revision_id uuid,
 PRIMARY KEY(provenance_id,kind,id)
);
CREATE TABLE eng_import_plan (
 id uuid PRIMARY KEY,revision_id uuid NOT NULL UNIQUE REFERENCES eng_revision(id) ON DELETE CASCADE,
 source_id uuid NOT NULL REFERENCES eng_source(id) ON DELETE CASCADE,project_id text NOT NULL,
 version integer NOT NULL,summary text NOT NULL,previous_plan_id uuid
);
CREATE TABLE eng_module_instance (
 plan_id uuid NOT NULL REFERENCES eng_import_plan(id) ON DELETE CASCADE,id uuid NOT NULL,
 definition_id text NOT NULL,definition_version integer NOT NULL,state text NOT NULL,
 applicability_reason text NOT NULL,head_revision_id uuid NOT NULL,PRIMARY KEY(plan_id,id)
);
CREATE TABLE eng_module_input (
 plan_id uuid NOT NULL,instance_id uuid NOT NULL,name text NOT NULL,producer_instance_id uuid,
 output text NOT NULL,revision_id uuid,digest text,required boolean NOT NULL,available boolean NOT NULL,
 PRIMARY KEY(plan_id,instance_id,name),FOREIGN KEY(plan_id,instance_id) REFERENCES eng_module_instance(plan_id,id) ON DELETE CASCADE
);
CREATE TABLE eng_workbook_region (
 revision_id uuid NOT NULL REFERENCES eng_revision(id) ON DELETE CASCADE,id uuid NOT NULL,
 source_id uuid NOT NULL REFERENCES eng_source(id) ON DELETE CASCADE,sheet text NOT NULL,
 start_row integer NOT NULL,end_row integer NOT NULL,start_column integer NOT NULL,end_column integer NOT NULL,
 kind text NOT NULL,semantic_label text NOT NULL,header_rows integer[] NOT NULL,PRIMARY KEY(revision_id,id)
);
CREATE TABLE eng_region_binding (
 revision_id uuid NOT NULL,region_id uuid NOT NULL,position integer NOT NULL,record_index integer,
 field text NOT NULL,sheet text NOT NULL,cell text,column_number integer,unit text NOT NULL,constant_value text,evidence_cells text[] NOT NULL,
 PRIMARY KEY(revision_id,region_id,position),FOREIGN KEY(revision_id,region_id) REFERENCES eng_workbook_region(revision_id,id) ON DELETE CASCADE
);
CREATE TABLE eng_semantic_fact (
 revision_id uuid NOT NULL,id uuid NOT NULL,region_id uuid NOT NULL,subject_id uuid,predicate text NOT NULL,
 value_kind text NOT NULL CHECK(value_kind IN('text','decimal','boolean','reference')),text_value text,decimal_value numeric,boolean_value boolean,reference_value uuid,unit text,
 claim_status text NOT NULL,maturity text NOT NULL,provenance_id uuid NOT NULL,
 PRIMARY KEY(revision_id,id),FOREIGN KEY(revision_id,region_id) REFERENCES eng_workbook_region(revision_id,id) ON DELETE CASCADE,
 CHECK(num_nonnulls(text_value,decimal_value,boolean_value,reference_value)=1)
);
ALTER TABLE eng_evidence ADD COLUMN source_id uuid REFERENCES eng_source(id);
UPDATE eng_evidence e SET source_id=r.source_id FROM eng_revision r WHERE r.id=e.revision_id;
ALTER TABLE eng_evidence ALTER COLUMN source_id SET NOT NULL;
INSERT INTO schema_migration(version) VALUES(34);
