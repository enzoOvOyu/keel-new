package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
	"math/big"
)

// A caller cannot attach another project's run, an arbitrary value, or a failed
// calculation to a field and have it represented as a calculated engineering fact.
func verifyEngineeringResults(ctx context.Context, tx pgx.Tx, source, project string, x engineering.Extraction) error {
	type calculated struct {
		entity   string
		run      string
		quantity engineering.Quantity
	}
	values := []calculated{}
	runs := map[string]bool{}
	for _, ev := range x.Evidence {
		if ev.CalculationRunID != nil {
			runs[*ev.CalculationRunID] = true
		}
	}
	for _, c := range x.Model.Cables {
		for _, l := range []*engineering.LengthEvidence{c.Calculated, c.SelectedStock, c.Estimated, c.Confirmed, c.AsBuilt} {
			if l != nil && l.CalculationRunID != nil {
				values = append(values, calculated{c.EntityID, *l.CalculationRunID, l.Length})
				runs[*l.CalculationRunID] = true
			}
		}
		for _, r := range c.Route {
			if r.Distance != nil && r.CalculationRunID != nil {
				values = append(values, calculated{c.EntityID, *r.CalculationRunID, *r.Distance})
				runs[*r.CalculationRunID] = true
			}
		}
	}
	for _, p := range x.Model.PowerPlans {
		for _, r := range p.Racks {
			if r.CalculationRunID != nil {
				runs[*r.CalculationRunID] = true
				for _, q := range []*engineering.Quantity{r.PlannedLoad, r.Remaining} {
					if q != nil {
						values = append(values, calculated{p.EntityID, *r.CalculationRunID, *q})
					}
				}
			}
		}
	}
	for _, v := range x.Model.Materials {
		if v.RequiredCalculationRunID != nil {
			runs[*v.RequiredCalculationRunID] = true
			values = append(values, calculated{v.EntityID, *v.RequiredCalculationRunID, v.Required})
		}
		if v.CalculationRunID != nil && v.PurchaseGap != nil {
			runs[*v.CalculationRunID] = true
			values = append(values, calculated{v.EntityID, *v.CalculationRunID, *v.PurchaseGap})
		}
	}
	for _, v := range x.Model.Workloads {
		if v.QuantityCalculationRunID != nil {
			runs[*v.QuantityCalculationRunID] = true
			values = append(values, calculated{v.EntityID, *v.QuantityCalculationRunID, v.Quantity})
		}
		if v.CalculationRunID != nil && v.LaborHours != nil {
			runs[*v.CalculationRunID] = true
			values = append(values, calculated{v.EntityID, *v.CalculationRunID, *v.LaborHours})
		}
	}

	outputs := map[string][]engineering.TypedValue{}
	for run := range runs {
		var ok bool
		if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_calculation_run WHERE id::text=$1 AND project_id=$2 AND source_id=$3 AND status IN('succeeded','partial'))`, run, project, source).Scan(&ok); e != nil {
			return e
		}
		if !ok {
			return errors.New("CALC_REFERENCE_UNAVAILABLE")
		}
		rows, e := tx.Query(ctx, `SELECT value FROM eng_calculation_value WHERE run_id=$1 AND direction='output'`, run)
		if e != nil {
			return e
		}
		for rows.Next() {
			var raw []byte
			var v engineering.TypedValue
			if e = rows.Scan(&raw); e == nil {
				e = json.Unmarshal(raw, &v)
			}
			if e != nil {
				break
			}
			outputs[run] = append(outputs[run], v)
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			return e
		}
	}
	for _, value := range values {
		found := false
		for _, v := range outputs[value.run] {
			if v.Kind == "decimal" && v.Decimal != nil && v.Unit != nil && engineering.EqualQuantities(value.quantity, engineering.Quantity{Value: *v.Decimal, Unit: *v.Unit}) {
				found = true
				break
			}
		}
		if !found {
			for _, ev := range x.Evidence {
				if ev.EntityID != value.entity || !ev.Formula || ev.CalculationRunID == nil || *ev.CalculationRunID != value.run {
					continue
				}
				var raw []byte
				e := tx.QueryRow(ctx, `SELECT recalculated_value FROM eng_calculation_formula WHERE run_id=$1 AND sheet=$2 AND cell=$3 AND compatibility='compatible' AND error_code IS NULL`, value.run, ev.Sheet, ev.Cell).Scan(&raw)
				if errors.Is(e, pgx.ErrNoRows) {
					continue
				}
				if e != nil {
					return e
				}
				var v engineering.TypedValue
				if e = json.Unmarshal(raw, &v); e != nil {
					return e
				}
				if v.Decimal != nil {
					a, oa := new(big.Rat).SetString(*v.Decimal)
					b, ob := new(big.Rat).SetString(value.quantity.Value)
					if oa && ob && a.Cmp(b) == 0 {
						found = true
						break
					}
				}
			}
		}
		if !found {
			return errors.New("CALC_VALUE_DOES_NOT_MATCH_RESULT")
		}
	}
	return nil
}
