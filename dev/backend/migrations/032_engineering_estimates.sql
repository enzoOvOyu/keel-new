BEGIN;
ALTER TABLE eng_target DROP CONSTRAINT eng_target_target_check;
ALTER TABLE eng_target ADD CONSTRAINT eng_target_target_check CHECK(target IN('CMDB_PLANNED_EQUIPMENT','CMDB_PLANNED_FACILITY','CMDB_PLANNED_PLACEMENT','CMDB_PLANNED_CONNECTIVITY','ENGINEERING_CABLING','ENGINEERING_POWER','ENGINEERING_MATERIAL','ENGINEERING_WORKLOAD'));
CREATE TABLE eng_material_estimate (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,spec_ref uuid NOT NULL,source_model_label text NOT NULL,
 required_value numeric NOT NULL,required_unit text NOT NULL,purchased_value numeric,purchased_unit text,spares_value numeric,spares_unit text,gap_value numeric,gap_unit text,
 calculation_run_id uuid REFERENCES eng_calculation_run(id),required_calculation_run_id uuid REFERENCES eng_calculation_run(id),
 PRIMARY KEY(revision_id,entity_id),FOREIGN KEY(revision_id,entity_id) REFERENCES eng_entity_version(revision_id,entity_id) ON DELETE CASCADE
);
CREATE TABLE eng_workload_estimate (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,work_type text NOT NULL,spec_ref uuid,source_model_label text NOT NULL,
 quantity_value numeric NOT NULL,quantity_unit text NOT NULL,length_value numeric,length_unit text,labor_value numeric,labor_unit text,
 calculation_run_id uuid REFERENCES eng_calculation_run(id),quantity_calculation_run_id uuid REFERENCES eng_calculation_run(id),
 PRIMARY KEY(revision_id,entity_id),FOREIGN KEY(revision_id,entity_id) REFERENCES eng_entity_version(revision_id,entity_id) ON DELETE CASCADE
);
INSERT INTO schema_migration(version) VALUES(32);
COMMIT;
