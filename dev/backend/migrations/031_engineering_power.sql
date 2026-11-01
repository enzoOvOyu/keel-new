BEGIN;
ALTER TABLE eng_target DROP CONSTRAINT eng_target_target_check;
ALTER TABLE eng_target ADD CONSTRAINT eng_target_target_check CHECK(target IN('CMDB_PLANNED_EQUIPMENT','CMDB_PLANNED_FACILITY','CMDB_PLANNED_PLACEMENT','CMDB_PLANNED_CONNECTIVITY','ENGINEERING_CABLING','ENGINEERING_POWER'));
CREATE TABLE eng_power_plan (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,
 PRIMARY KEY(revision_id,entity_id),FOREIGN KEY(revision_id,entity_id) REFERENCES eng_entity_version(revision_id,entity_id) ON DELETE CASCADE
);
CREATE TABLE eng_equipment_power (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,equipment_id uuid NOT NULL,basis text NOT NULL,
 power_value numeric NOT NULL,power_unit text NOT NULL,cooling_type text NOT NULL,evidence_ids uuid[] NOT NULL,
 PRIMARY KEY(revision_id,entity_id,equipment_id,basis),FOREIGN KEY(revision_id,entity_id) REFERENCES eng_power_plan(revision_id,entity_id) ON DELETE CASCADE
);
CREATE TABLE eng_rack_power (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,rack_id uuid NOT NULL,
 capacity_value numeric,capacity_unit text,cooling_value numeric,cooling_unit text,cooling_type text NOT NULL,
 load_value numeric,load_unit text,remaining_value numeric,remaining_unit text,calculation_run_id uuid REFERENCES eng_calculation_run(id),
 PRIMARY KEY(revision_id,entity_id,rack_id),FOREIGN KEY(revision_id,entity_id) REFERENCES eng_power_plan(revision_id,entity_id) ON DELETE CASCADE
);
INSERT INTO schema_migration(version) VALUES(31);
COMMIT;
