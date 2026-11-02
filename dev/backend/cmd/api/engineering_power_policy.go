package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"keel.local/dev/internal/engineering"
)

func (s *Server) prepareStandardPowerRules(ctx context.Context, inputs []engineering.InputPreviewRequest, actor string) error {
	definitions := []engineering.CalculationRule{}
	for _, input := range inputs {
		rules, e := engineering.PowerRuleDefinitions(s.project, input)
		if e != nil {
			return e
		}
		definitions = append(definitions, rules...)
	}
	for _, rule := range definitions {
		existing, e := engineeringRule(ctx, s.db, s.project, rule.ID, rule.Version)
		if e == nil {
			if engineering.Digest([]any{existing.Rule.Expression, existing.Rule.Inputs, existing.Rule.Outputs, existing.Rule.ScopeType, existing.Rule.ScopeID}) != engineering.Digest([]any{rule.Expression, rule.Inputs, rule.Outputs, rule.ScopeType, rule.ScopeID}) {
				return errors.New("STEP_POWER_RULE_DEFINITION_CONFLICT")
			}
			continue
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		if _, e = s.saveEngineeringRule(ctx, engineering.RuleDraft{ExpectedVersion: rule.Version - 1, Rule: rule, Reason: "Explicit power rule candidate from standard input; requires rule review"}, actor); e != nil {
			return e
		}
	}
	return nil
}
func powerPolicyRules(ctx context.Context, q engineeringQuery, project string, m engineering.PhysicalModel) (map[string]engineering.CalculationRule, []engineering.ValidationIssue, error) {
	rules := map[string]engineering.CalculationRule{}
	issues := []engineering.ValidationIssue{}
	for _, p := range m.PowerPlans {
		if p.Scope == nil {
			continue
		}
		refs := []engineering.VersionReference{p.Scope.DemandRule}
		for _, s := range p.Supplies {
			refs = append(refs, s.AllocationRule)
		}
		for _, ref := range refs {
			key := ref.ID + "@" + fmt.Sprint(ref.Version)
			if _, ok := rules[key]; ok {
				continue
			}
			view, e := engineeringRule(ctx, q, project, ref.ID, ref.Version)
			code := ""
			if errors.Is(e, pgx.ErrNoRows) {
				code = "POWER_RULE_UNAVAILABLE"
			} else if e != nil {
				return nil, nil, e
			} else if view.Rule.ApprovalStatus != "approved" {
				code = "POWER_RULE_APPROVAL_REQUIRED"
			} else if view.Rule.Domain != engineering.PowerDomain || view.Rule.ScopeType != "project" || view.Rule.ScopeID != project {
				code = "POWER_RULE_SCOPE_INVALID"
			}
			if code != "" {
				issues = append(issues, engineering.ValidationIssue{ID: engineering.StableID(project, "power-rule-issue", p.EntityID+key+code), EntityID: p.EntityID, ProjectID: project, Code: code, Severity: "BLOCK", ModuleID: "power_cooling", Field: "scope", Message: "电力计算需要当前项目已批准的规则版本：" + key, PublishTarget: engineering.PowerTarget})
				continue
			}
			rules[key] = view.Rule
		}
	}
	return rules, issues, nil
}
func (s *Server) scopedPowerBalance(ctx context.Context, m engineering.PhysicalModel, revision string) (engineering.CalculationRun, error) {
	rules, issues, e := powerPolicyRules(ctx, s.db, s.project, m)
	if e != nil {
		return engineering.CalculationRun{}, e
	}
	if len(issues) > 0 {
		run := engineering.PowerBalance(engineering.EmptyPhysical(m.ProjectID), revision)
		run.Status = "failed"
		run.Errors = []engineering.CalculationError{{Code: issues[0].Code, Message: issues[0].Message}}
		return run, errors.New(issues[0].Code)
	}
	return engineering.ScopedPowerBalance(m, revision, rules)
}

// A calculated value must come from this scoped plan's current inputs and the
// exact output name, not merely equal any number in an unrelated calculation.
func verifyScopedPowerOutputs(ctx context.Context, q engineeringQuery, m engineering.PhysicalModel) error {
	digest := powerInputDigest(m)
	snapshots := map[string]string{}
	for _, p := range m.PowerPlans {
		if p.Scope == nil {
			continue
		}
		check := func(run *string, name string, value *engineering.Quantity) error {
			if run == nil || value == nil {
				return nil
			}
			var provider, version, revision string
			var raw []byte
			if e := q.QueryRow(ctx, `SELECT r.provider_id,r.provider_version,r.revision_id::text,v.value FROM eng_calculation_run r JOIN eng_calculation_value v ON v.run_id=r.id AND v.direction='output' AND v.name=$2 WHERE r.id::text=$1 AND r.project_id=$3`, *run, name, m.ProjectID).Scan(&provider, &version, &revision, &raw); e != nil {
				return errors.New("CALC_POWER_OUTPUT_UNAVAILABLE")
			}
			if _, ok := snapshots[*run]; !ok {
				snapshot, _, e := engineeringModel(ctx, q, revision, m.ProjectID)
				if e != nil {
					return e
				}
				snapshots[*run] = powerInputDigest(snapshot)
			}
			if provider != "engineering-power-balance" || version != "2" || snapshots[*run] != digest {
				return errors.New("CALC_POWER_INPUT_STALE")
			}
			var actual engineering.TypedValue
			if json.Unmarshal(raw, &actual) != nil || actual.Decimal == nil || actual.Unit == nil || !engineering.EqualQuantities(*value, engineering.Quantity{Value: *actual.Decimal, Unit: *actual.Unit}) {
				return errors.New("CALC_OUTPUT_VALUE_MISMATCH")
			}
			return nil
		}
		for _, r := range p.Racks {
			if e := check(r.CalculationRunID, r.RackID+"/load", r.PlannedLoad); e != nil {
				return e
			}
			if e := check(r.CalculationRunID, r.RackID+"/remaining", r.Remaining); e != nil {
				return e
			}
		}
		for _, c := range p.Circuits {
			if e := check(c.CalculationRunID, c.ID+"/load", c.Load); e != nil {
				return e
			}
			if e := check(c.CalculationRunID, c.ID+"/remaining", c.Remaining); e != nil {
				return e
			}
		}
		for _, s := range p.Supplies {
			if e := check(s.CalculationRunID, s.ID+"/load", s.Load); e != nil {
				return e
			}
		}
	}
	return nil
}
