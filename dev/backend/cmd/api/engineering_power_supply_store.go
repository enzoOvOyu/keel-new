package main

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
)

func loadPowerSupplyRows(ctx context.Context, q engineeringQuery, revision string, p *engineering.PowerPlan) error {
	var raw []byte
	if e := q.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(jsonb_strip_nulls(jsonb_build_object('id',id,'code',code,'object_code',object_code,'location_id',location_id,'upstream_code',upstream_code,'capacity',jsonb_build_object('value',capacity_value::text,'unit',capacity_unit),'load',CASE WHEN load_value IS NOT NULL THEN jsonb_build_object('value',load_value::text,'unit',load_unit) END,'remaining',CASE WHEN remaining_value IS NOT NULL THEN jsonb_build_object('value',remaining_value::text,'unit',remaining_unit) END,'calculation_run_id',calculation_run_id)) ORDER BY id),'[]'::jsonb) FROM eng_power_circuit WHERE revision_id=$1 AND entity_id=$2`, revision, p.EntityID).Scan(&raw); e != nil {
		return e
	}
	if e := json.Unmarshal(raw, &p.Circuits); e != nil {
		return e
	}
	if e := q.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(jsonb_strip_nulls(jsonb_build_object('id',id,'equipment_id',equipment_id,'psu',psu,'circuit_code',circuit_code,'feed_group',feed_group,'allocation_rule',jsonb_build_object('id',rule_id,'version',rule_version),'load',CASE WHEN load_value IS NOT NULL THEN jsonb_build_object('value',load_value::text,'unit',load_unit) END,'calculation_run_id',calculation_run_id)) ORDER BY id),'[]'::jsonb) FROM eng_equipment_supply WHERE revision_id=$1 AND entity_id=$2`, revision, p.EntityID).Scan(&raw); e != nil {
		return e
	}
	return json.Unmarshal(raw, &p.Supplies)
}
func savePowerSupplyRows(ctx context.Context, tx pgx.Tx, revision string, p engineering.PowerPlan) error {
	for _, v := range p.Circuits {
		lv, lu := quantityColumns(v.Load)
		rv, ru := quantityColumns(v.Remaining)
		if _, e := tx.Exec(ctx, `INSERT INTO eng_power_circuit VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, revision, p.EntityID, v.ID, v.Code, v.ObjectCode, v.LocationID, v.UpstreamCode, v.Capacity.Value, v.Capacity.Unit, lv, lu, rv, ru, v.CalculationRunID); e != nil {
			return e
		}
	}
	for _, v := range p.Supplies {
		lv, lu := quantityColumns(v.Load)
		if _, e := tx.Exec(ctx, `INSERT INTO eng_equipment_supply VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, revision, p.EntityID, v.ID, v.EquipmentID, v.PSU, v.CircuitCode, v.FeedGroup, v.AllocationRule.ID, v.AllocationRule.Version, lv, lu, v.CalculationRunID); e != nil {
			return e
		}
	}
	return nil
}
