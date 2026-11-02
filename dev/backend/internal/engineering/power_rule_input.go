package engineering

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// PowerRuleDefinitions proposes versioned registry definitions. It never grants
// approval; the existing rule review API remains the authority.
func PowerRuleDefinitions(project string, q InputPreviewRequest) ([]CalculationRule, error) {
	if q.TableID != "power_rules" {
		return nil, nil
	}
	preview := PreviewStepInput(q)
	if !preview.Valid {
		return nil, fmt.Errorf("STEP_INPUT_INVALID")
	}
	out := []CalculationRule{}
	seen := map[string]string{}
	for _, row := range preview.Rows {
		v := append([]string{}, row...)
		for i := range v {
			v[i] = strings.TrimSpace(v[i])
		}
		version, e := strconv.Atoi(v[1])
		if e != nil || version < 1 || !decimal(v[4]) {
			return nil, fmt.Errorf("STEP_POWER_RULE_INVALID")
		}
		factor, _ := new(big.Rat).SetString(v[4])
		if factor.Sign() < 0 {
			return nil, fmt.Errorf("STEP_POWER_RULE_INVALID")
		}
		ref, e := PowerRuleReference(project, v[0]+"@"+v[1])
		if e != nil {
			return nil, e
		}
		input := "power"
		quantity := Quantity{Value: v[4], Unit: "1"}
		expected, _ := resultValue(measured{new(big.Rat).Mul(big.NewRat(1000, 1), factor), "power"}, "W")
		source := "1000"
		unit := "W"
		rule := CalculationRule{ID: ref.ID, Version: version, Domain: PowerDomain, ScopeType: "project", ScopeID: project, ApprovalStatus: "draft", Inputs: []RuleIO{{Name: "power", ValueType: "decimal", Unit: "W", Required: true}}, Outputs: []RuleIO{{Name: "power", ValueType: "decimal", Unit: "W", Required: true}}, Expression: RuleExpression{Operator: "multiply", Arguments: []RuleExpression{{Operator: "input", Input: &input}, {Operator: "literal", Literal: &quantity}}}, Dependencies: []VersionReference{}, ValidationCases: []RuleCase{{ID: "factor-1000W", Inputs: []NamedValue{{Name: "power", Value: TypedValue{Kind: "decimal", Decimal: &source, Unit: &unit}}}, Expected: []NamedValue{{Name: "power", Value: expected}}, Tolerance: "0"}}}
		key := ref.ID + "@" + v[1]
		digest := Digest(rule.Expression)
		if previous, ok := seen[key]; ok {
			if previous != digest {
				return nil, fmt.Errorf("STEP_POWER_RULE_DEFINITION_CONFLICT")
			}
			continue
		}
		seen[key] = digest
		out = append(out, rule)
	}
	return out, nil
}
