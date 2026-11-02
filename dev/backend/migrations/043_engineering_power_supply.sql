BEGIN;
ALTER TABLE eng_power_plan ADD COLUMN scenario text,ADD COLUMN basis text,ADD COLUMN demand_rule_id uuid,ADD COLUMN demand_rule_version integer,ADD COLUMN redundancy_mode text;
ALTER TABLE eng_rack_power ADD COLUMN capacity_basis text NOT NULL DEFAULT '';
CREATE TABLE eng_power_circuit (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,id uuid NOT NULL,code text NOT NULL,object_code text NOT NULL,location_id uuid NOT NULL,upstream_code text NOT NULL,
 capacity_value numeric NOT NULL,capacity_unit text NOT NULL,load_value numeric,load_unit text,remaining_value numeric,remaining_unit text,calculation_run_id uuid REFERENCES eng_calculation_run(id),
 PRIMARY KEY(revision_id,entity_id,id),UNIQUE(revision_id,entity_id,code),FOREIGN KEY(revision_id,entity_id) REFERENCES eng_power_plan(revision_id,entity_id) ON DELETE CASCADE
);
CREATE TABLE eng_equipment_supply (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,id uuid NOT NULL,equipment_id uuid NOT NULL,psu text NOT NULL,circuit_code text NOT NULL,feed_group text NOT NULL,
 rule_id uuid NOT NULL,rule_version integer NOT NULL,load_value numeric,load_unit text,calculation_run_id uuid REFERENCES eng_calculation_run(id),
 PRIMARY KEY(revision_id,entity_id,id),UNIQUE(revision_id,entity_id,equipment_id,psu),FOREIGN KEY(revision_id,entity_id) REFERENCES eng_power_plan(revision_id,entity_id) ON DELETE CASCADE
);
INSERT INTO schema_migration(version) VALUES(43);
COMMIT;
