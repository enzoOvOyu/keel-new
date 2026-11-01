BEGIN;
ALTER TABLE eng_target DROP CONSTRAINT eng_target_target_check;
ALTER TABLE eng_target ADD CONSTRAINT eng_target_target_check CHECK(target IN('CMDB_PLANNED_EQUIPMENT','CMDB_PLANNED_FACILITY','CMDB_PLANNED_PLACEMENT','CMDB_PLANNED_CONNECTIVITY','ENGINEERING_CABLING','ENGINEERING_POWER','ENGINEERING_MATERIAL','ENGINEERING_WORKLOAD','NETWORK_REVISION'));
CREATE TABLE eng_network_plan(revision_id uuid NOT NULL,entity_id uuid NOT NULL,physical_revision_id uuid REFERENCES eng_revision(id),source_ids uuid[] NOT NULL,
 PRIMARY KEY(revision_id,entity_id),FOREIGN KEY(revision_id,entity_id) REFERENCES eng_entity_version(revision_id,entity_id) ON DELETE CASCADE);
CREATE TABLE eng_network_fabrics(revision_id uuid NOT NULL,plan_id uuid NOT NULL,position integer NOT NULL,id uuid,name text,kind text,evidence_ids uuid[],
 PRIMARY KEY(revision_id,plan_id,position),FOREIGN KEY(revision_id,plan_id) REFERENCES eng_network_plan(revision_id,entity_id) ON DELETE CASCADE);
CREATE TABLE eng_network_planes(revision_id uuid NOT NULL,plan_id uuid NOT NULL,position integer NOT NULL,id uuid,fabric_id uuid,name text,evidence_ids uuid[],
 PRIMARY KEY(revision_id,plan_id,position),FOREIGN KEY(revision_id,plan_id) REFERENCES eng_network_plan(revision_id,entity_id) ON DELETE CASCADE);
CREATE TABLE eng_network_device_roles(revision_id uuid NOT NULL,plan_id uuid NOT NULL,position integer NOT NULL,equipment_id uuid,plane_id uuid,role text,evidence_ids uuid[],
 PRIMARY KEY(revision_id,plan_id,position),FOREIGN KEY(revision_id,plan_id) REFERENCES eng_network_plan(revision_id,entity_id) ON DELETE CASCADE);
CREATE TABLE eng_network_interface_mappings(revision_id uuid NOT NULL,plan_id uuid NOT NULL,position integer NOT NULL,id uuid,physical_port_id uuid,plane_id uuid,logical_name text,evidence_ids uuid[],
 PRIMARY KEY(revision_id,plan_id,position),FOREIGN KEY(revision_id,plan_id) REFERENCES eng_network_plan(revision_id,entity_id) ON DELETE CASCADE);
CREATE TABLE eng_network_allocations(revision_id uuid NOT NULL,plan_id uuid NOT NULL,position integer NOT NULL,id uuid,kind text,interface_id uuid,entity_id uuid,address text,prefix_length integer,asn bigint,vrf_ref text,evidence_ids uuid[],
 PRIMARY KEY(revision_id,plan_id,position),FOREIGN KEY(revision_id,plan_id) REFERENCES eng_network_plan(revision_id,entity_id) ON DELETE CASCADE);
CREATE TABLE eng_network_bgp(revision_id uuid NOT NULL,plan_id uuid NOT NULL,position integer NOT NULL,id uuid,a_interface_id uuid,z_interface_id uuid,local_asn_allocation_id uuid,peer_asn_allocation_id uuid,afi text,safi text,policy_ref text,evidence_ids uuid[],
 PRIMARY KEY(revision_id,plan_id,position),FOREIGN KEY(revision_id,plan_id) REFERENCES eng_network_plan(revision_id,entity_id) ON DELETE CASCADE);
CREATE TABLE eng_network_bfd(revision_id uuid NOT NULL,plan_id uuid NOT NULL,position integer NOT NULL,session_id uuid,min_tx_milliseconds integer,min_rx_milliseconds integer,multiplier integer,evidence_ids uuid[],
 PRIMARY KEY(revision_id,plan_id,position),FOREIGN KEY(revision_id,plan_id) REFERENCES eng_network_plan(revision_id,entity_id) ON DELETE CASCADE);
CREATE TABLE eng_network_qos(revision_id uuid NOT NULL,plan_id uuid NOT NULL,position integer NOT NULL,id uuid,interface_ids uuid[],roce_version integer,lossless_priorities integer[],ecn_enabled boolean,pfc_enabled boolean,policy_ref text,evidence_ids uuid[],
 PRIMARY KEY(revision_id,plan_id,position),FOREIGN KEY(revision_id,plan_id) REFERENCES eng_network_plan(revision_id,entity_id) ON DELETE CASCADE);
CREATE TABLE eng_network_adaptive_routing(revision_id uuid NOT NULL,plan_id uuid NOT NULL,position integer NOT NULL,fabric_id uuid,enabled boolean,policy_ref text,evidence_ids uuid[],
 PRIMARY KEY(revision_id,plan_id,position),FOREIGN KEY(revision_id,plan_id) REFERENCES eng_network_plan(revision_id,entity_id) ON DELETE CASCADE);
CREATE TABLE eng_network_evpn_vxlan(revision_id uuid NOT NULL,plan_id uuid NOT NULL,position integer NOT NULL,id uuid,fabric_id uuid,vni integer,vlan integer,vrf_ref text,vtep_allocation_ids uuid[],route_distinguisher text,import_targets text[],export_targets text[],evidence_ids uuid[],
 PRIMARY KEY(revision_id,plan_id,position),FOREIGN KEY(revision_id,plan_id) REFERENCES eng_network_plan(revision_id,entity_id) ON DELETE CASCADE);
INSERT INTO schema_migration(version) VALUES(33);
COMMIT;
