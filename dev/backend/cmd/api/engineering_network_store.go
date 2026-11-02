package main

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
)

// These collections and SQL columns are fixed domain contracts, never worksheet-defined schemas.
type networkCollection struct {
	table        string
	columns      string
	record       string
	selectValues string
	value        any
}

func networkCollections(p *engineering.NetworkPlan) []networkCollection {
	return []networkCollection{
		{"eng_network_fabrics", "id,name,kind,evidence_ids", "id text,name text,kind text,evidence_ids text[]", `NULLIF(v.id,'')::uuid,v.name,v.kind,ARRAY(SELECT NULLIF(x,'')::uuid FROM unnest(v.evidence_ids) x)`, &p.Fabrics},
		{"eng_network_planes", "id,fabric_id,name,evidence_ids,profile", "id text,fabric_id text,name text,evidence_ids text[],profile text", `NULLIF(v.id,'')::uuid,NULLIF(v.fabric_id,'')::uuid,v.name,ARRAY(SELECT NULLIF(x,'')::uuid FROM unnest(v.evidence_ids) x),COALESCE(v.profile,'')`, &p.Planes},
		{"eng_network_device_roles", "equipment_id,plane_id,role,evidence_ids", "equipment_id text,plane_id text,role text,evidence_ids text[]", `NULLIF(v.equipment_id,'')::uuid,NULLIF(v.plane_id,'')::uuid,v.role,ARRAY(SELECT NULLIF(x,'')::uuid FROM unnest(v.evidence_ids) x)`, &p.DeviceRoles},
		{"eng_network_interface_mappings", "id,physical_port_id,plane_id,logical_name,evidence_ids", "id text,physical_port_id text,plane_id text,logical_name text,evidence_ids text[]", `NULLIF(v.id,'')::uuid,NULLIF(v.physical_port_id,'')::uuid,NULLIF(v.plane_id,'')::uuid,v.logical_name,ARRAY(SELECT NULLIF(x,'')::uuid FROM unnest(v.evidence_ids) x)`, &p.InterfaceMappings},
		{"eng_network_allocations", "id,kind,interface_id,entity_id,address,prefix_length,asn,vrf_ref,evidence_ids", "id text,kind text,interface_id text,entity_id text,address text,prefix_length integer,asn bigint,vrf_ref text,evidence_ids text[]", `NULLIF(v.id,'')::uuid,v.kind,NULLIF(v.interface_id,'')::uuid,NULLIF(v.entity_id,'')::uuid,v.address,v.prefix_length,v.asn,v.vrf_ref,ARRAY(SELECT NULLIF(x,'')::uuid FROM unnest(v.evidence_ids) x)`, &p.Allocations},
		{"eng_network_bgp", "id,a_interface_id,z_interface_id,local_asn_allocation_id,peer_asn_allocation_id,afi,safi,policy_ref,evidence_ids", "id text,a_interface_id text,z_interface_id text,local_asn_allocation_id text,peer_asn_allocation_id text,afi text,safi text,policy_ref text,evidence_ids text[]", `NULLIF(v.id,'')::uuid,NULLIF(v.a_interface_id,'')::uuid,NULLIF(v.z_interface_id,'')::uuid,NULLIF(v.local_asn_allocation_id,'')::uuid,NULLIF(v.peer_asn_allocation_id,'')::uuid,v.afi,v.safi,v.policy_ref,ARRAY(SELECT NULLIF(x,'')::uuid FROM unnest(v.evidence_ids) x)`, &p.BGP},
		{"eng_network_bfd", "session_id,min_tx_milliseconds,min_rx_milliseconds,multiplier,evidence_ids", "session_id text,min_tx_milliseconds integer,min_rx_milliseconds integer,multiplier integer,evidence_ids text[]", `NULLIF(v.session_id,'')::uuid,v.min_tx_milliseconds,v.min_rx_milliseconds,v.multiplier,ARRAY(SELECT NULLIF(x,'')::uuid FROM unnest(v.evidence_ids) x)`, &p.BFD},
		{"eng_network_qos", "id,interface_ids,roce_version,lossless_priorities,ecn_enabled,pfc_enabled,policy_ref,evidence_ids", "id text,interface_ids text[],roce_version integer,lossless_priorities integer[],ecn_enabled boolean,pfc_enabled boolean,policy_ref text,evidence_ids text[]", `NULLIF(v.id,'')::uuid,ARRAY(SELECT NULLIF(x,'')::uuid FROM unnest(v.interface_ids) x),v.roce_version,v.lossless_priorities,v.ecn_enabled,v.pfc_enabled,v.policy_ref,ARRAY(SELECT NULLIF(x,'')::uuid FROM unnest(v.evidence_ids) x)`, &p.QoS},
		{"eng_network_adaptive_routing", "fabric_id,enabled,policy_ref,evidence_ids", "fabric_id text,enabled boolean,policy_ref text,evidence_ids text[]", `NULLIF(v.fabric_id,'')::uuid,v.enabled,v.policy_ref,ARRAY(SELECT NULLIF(x,'')::uuid FROM unnest(v.evidence_ids) x)`, &p.AdaptiveRouting},
		{"eng_network_evpn_vxlan", "id,fabric_id,vni,vlan,vrf_ref,vtep_allocation_ids,route_distinguisher,import_targets,export_targets,evidence_ids", "id text,fabric_id text,vni integer,vlan integer,vrf_ref text,vtep_allocation_ids text[],route_distinguisher text,import_targets text[],export_targets text[],evidence_ids text[]", `NULLIF(v.id,'')::uuid,NULLIF(v.fabric_id,'')::uuid,v.vni,v.vlan,v.vrf_ref,ARRAY(SELECT NULLIF(x,'')::uuid FROM unnest(v.vtep_allocation_ids) x),v.route_distinguisher,v.import_targets,v.export_targets,ARRAY(SELECT NULLIF(x,'')::uuid FROM unnest(v.evidence_ids) x)`, &p.EVPNVXLAN},
	}
}
func loadNetworkRows(ctx context.Context, q engineeringQuery, revision string, m *engineering.PhysicalModel) error {
	var raw []byte
	e := q.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(jsonb_build_object('entity_id',p.entity_id,'revision_id',p.revision_id,'physical_revision_id',p.physical_revision_id,'source_ids',p.source_ids,'provenance_id',v.provenance_id,'maturity',v.maturity,'attributes','[]'::jsonb,'evidence_ids',COALESCE((SELECT jsonb_agg(id) FROM eng_evidence e WHERE e.revision_id=p.revision_id AND e.entity_id=p.entity_id),'[]'::jsonb)) ORDER BY p.entity_id),'[]'::jsonb) FROM eng_network_plan p JOIN eng_entity_version v USING(revision_id,entity_id) WHERE p.revision_id=$1`, revision).Scan(&raw)
	if e != nil {
		return e
	}
	if e = json.Unmarshal(raw, &m.NetworkPlans); e != nil {
		return e
	}
	for i := range m.NetworkPlans {
		p := &m.NetworkPlans[i]
		for _, c := range networkCollections(p) {
			sql := `SELECT COALESCE(jsonb_agg(to_jsonb(t)-'revision_id'-'plan_id'-'position' ORDER BY position),'[]'::jsonb) FROM ` + c.table + ` t WHERE revision_id=$1 AND plan_id=$2`
			if e = q.QueryRow(ctx, sql, revision, p.EntityID).Scan(&raw); e != nil {
				return e
			}
			if e = json.Unmarshal(raw, c.value); e != nil {
				return e
			}
		}
	}
	return nil
}
func saveNetworkRows(ctx context.Context, tx pgx.Tx, revision string, m *engineering.PhysicalModel, version func(*engineering.EntityVersion, string) error) error {
	for i := range m.NetworkPlans {
		p := &m.NetworkPlans[i]
		if e := version(&p.EntityVersion, "network_plan"); e != nil {
			return e
		}
		if p.SourceIDs == nil {
			p.SourceIDs = []string{}
		}
		if _, e := tx.Exec(ctx, `INSERT INTO eng_network_plan VALUES($1,$2,NULLIF($3,'')::uuid,$4)`, revision, p.EntityID, p.PhysicalRevisionID, p.SourceIDs); e != nil {
			return e
		}
		for j := range p.Fabrics {
			if len(p.Fabrics[j].EvidenceIDs) == 0 {
				p.Fabrics[j].EvidenceIDs = append([]string{}, p.EvidenceIDs...)
			}
		}
		for j := range p.Planes {
			if len(p.Planes[j].EvidenceIDs) == 0 {
				p.Planes[j].EvidenceIDs = append([]string{}, p.EvidenceIDs...)
			}
		}
		for j := range p.DeviceRoles {
			if len(p.DeviceRoles[j].EvidenceIDs) == 0 {
				p.DeviceRoles[j].EvidenceIDs = append([]string{}, p.EvidenceIDs...)
			}
		}
		for j := range p.InterfaceMappings {
			if len(p.InterfaceMappings[j].EvidenceIDs) == 0 {
				p.InterfaceMappings[j].EvidenceIDs = append([]string{}, p.EvidenceIDs...)
			}
		}
		for j := range p.Allocations {
			if len(p.Allocations[j].EvidenceIDs) == 0 {
				p.Allocations[j].EvidenceIDs = append([]string{}, p.EvidenceIDs...)
			}
		}
		for j := range p.BGP {
			if len(p.BGP[j].EvidenceIDs) == 0 {
				p.BGP[j].EvidenceIDs = append([]string{}, p.EvidenceIDs...)
			}
		}
		for j := range p.BFD {
			if len(p.BFD[j].EvidenceIDs) == 0 {
				p.BFD[j].EvidenceIDs = append([]string{}, p.EvidenceIDs...)
			}
		}
		for j := range p.QoS {
			if len(p.QoS[j].EvidenceIDs) == 0 {
				p.QoS[j].EvidenceIDs = append([]string{}, p.EvidenceIDs...)
			}
		}
		for j := range p.AdaptiveRouting {
			if len(p.AdaptiveRouting[j].EvidenceIDs) == 0 {
				p.AdaptiveRouting[j].EvidenceIDs = append([]string{}, p.EvidenceIDs...)
			}
		}
		for j := range p.EVPNVXLAN {
			if len(p.EVPNVXLAN[j].EvidenceIDs) == 0 {
				p.EVPNVXLAN[j].EvidenceIDs = append([]string{}, p.EvidenceIDs...)
			}
		}
		for _, c := range networkCollections(p) {
			// A transient typed JSON transport is split into relational columns; no network JSON document is stored.
			sql := `INSERT INTO ` + c.table + `(revision_id,plan_id,position,` + c.columns + `) SELECT $1,$2,x.position,` + c.selectValues + ` FROM jsonb_array_elements($3::jsonb) WITH ORDINALITY x(item,position) CROSS JOIN LATERAL jsonb_to_record(x.item) AS v(` + c.record + `)`
			raw := jsonBytes(c.value)
			if string(raw) == "null" {
				raw = []byte("[]")
			}
			if _, e := tx.Exec(ctx, sql, revision, p.EntityID, raw); e != nil {
				return e
			}
		}
	}
	return nil
}
