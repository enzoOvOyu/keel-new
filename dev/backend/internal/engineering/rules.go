package engineering

import (
	"fmt"
	"github.com/google/uuid"
	"math/big"
	"strings"
	"time"
)

type measured struct {
	value     *big.Rat
	dimension string
}

func unitScale(unit string) (string, *big.Rat) {
	dimension, scale := unit, "1"
	switch unit {
	case "1", "count":
		dimension = "scalar"
	case "%":
		dimension = "scalar"
		scale = "0.01"
	case "m":
		dimension = "length"
	case "cm":
		dimension = "length"
		scale = "0.01"
	case "mm":
		dimension = "length"
		scale = "0.001"
	case "in":
		dimension = "length"
		scale = "0.0254"
	case "ft":
		dimension = "length"
		scale = "0.3048"
	case "W":
		dimension = "power"
	case "kW":
		dimension = "power"
		scale = "1000"
	}
	n, _ := new(big.Rat).SetString(scale)
	return dimension, n
}
func measure(q Quantity) (measured, error) {
	if q.Validate() != nil {
		return measured{}, fmt.Errorf("RULE_QUANTITY_INVALID")
	}
	dim, scale := unitScale(q.Unit)
	n, _ := new(big.Rat).SetString(q.Value)
	return measured{n.Mul(n, scale), dim}, nil
}
func resultValue(v measured, unit string) (TypedValue, error) {
	dim, scale := unitScale(unit)
	if dim != v.dimension {
		return TypedValue{}, fmt.Errorf("RULE_UNIT_MISMATCH")
	}
	n := new(big.Rat).Quo(v.value, scale)
	value := strings.TrimRight(strings.TrimRight(n.FloatString(18), "0"), ".")
	if value == "" || value == "-0" {
		value = "0"
	}
	return TypedValue{Kind: "decimal", Decimal: &value, Unit: &unit}, nil
}
func evaluate(e RuleExpression, inputs map[string]measured, depth int) (measured, error) {
	if depth > 64 {
		return measured{}, fmt.Errorf("RULE_EXPRESSION_DEPTH")
	}
	fail := func() (measured, error) { return measured{}, fmt.Errorf("RULE_OPERATOR_OR_ARGUMENTS_INVALID") }
	if e.Operator == "input" {
		if e.Input == nil || e.Literal != nil || len(e.Arguments) != 0 {
			return fail()
		}
		v, ok := inputs[*e.Input]
		if !ok {
			return measured{}, fmt.Errorf("RULE_INPUT_MISSING")
		}
		return measured{new(big.Rat).Set(v.value), v.dimension}, nil
	}
	if e.Operator == "literal" {
		if e.Literal == nil || e.Input != nil || len(e.Arguments) != 0 {
			return fail()
		}
		return measure(*e.Literal)
	}
	if e.Input != nil || e.Literal != nil {
		return fail()
	}
	values := []measured{}
	for _, a := range e.Arguments {
		v, err := evaluate(a, inputs, depth+1)
		if err != nil {
			return measured{}, err
		}
		values = append(values, v)
	}
	if len(values) == 0 {
		return fail()
	}
	out := values[0]
	same := func() bool {
		for _, v := range values {
			if v.dimension != out.dimension {
				return false
			}
		}
		return true
	}
	switch e.Operator {
	case "add", "subtract", "min", "max":
		if !same() {
			return measured{}, fmt.Errorf("RULE_UNIT_MISMATCH")
		}
		if e.Operator == "subtract" && len(values) != 2 {
			return fail()
		}
		for _, v := range values[1:] {
			switch e.Operator {
			case "add":
				out.value.Add(out.value, v.value)
			case "subtract":
				out.value.Sub(out.value, v.value)
			case "min":
				if v.value.Cmp(out.value) < 0 {
					out.value.Set(v.value)
				}
			case "max":
				if v.value.Cmp(out.value) > 0 {
					out.value.Set(v.value)
				}
			}
		}
	case "multiply", "divide":
		if len(values) != 2 {
			return fail()
		}
		a, b := values[0], values[1]
		if e.Operator == "multiply" {
			if a.dimension == "scalar" {
				out.dimension = b.dimension
			} else if b.dimension != "scalar" {
				return measured{}, fmt.Errorf("RULE_COMPOUND_UNIT_UNSUPPORTED")
			}
			out.value.Mul(a.value, b.value)
		} else {
			if b.value.Sign() == 0 {
				return measured{}, fmt.Errorf("RULE_DIVIDE_BY_ZERO")
			}
			if a.dimension == b.dimension {
				out.dimension = "scalar"
			} else if b.dimension != "scalar" {
				return measured{}, fmt.Errorf("RULE_COMPOUND_UNIT_UNSUPPORTED")
			}
			out.value.Quo(a.value, b.value)
		}
	case "abs", "ceil", "floor":
		if len(values) != 1 {
			return fail()
		}
		if e.Operator != "abs" && out.dimension != "scalar" {
			return measured{}, fmt.Errorf("RULE_ROUNDING_STEP_REQUIRED")
		}
		if e.Operator == "abs" {
			out.value.Abs(out.value)
		} else {
			q, r := new(big.Int), new(big.Int)
			q.QuoRem(out.value.Num(), out.value.Denom(), r)
			if r.Sign() != 0 {
				if e.Operator == "ceil" && out.value.Sign() > 0 {
					q.Add(q, big.NewInt(1))
				}
				if e.Operator == "floor" && out.value.Sign() < 0 {
					q.Sub(q, big.NewInt(1))
				}
			}
			out.value.SetInt(q)
		}
	case "select_ceiling":
		if len(values) < 2 || !same() {
			return fail()
		}
		var choice *big.Rat
		for _, v := range values[1:] {
			if v.value.Cmp(out.value) >= 0 && (choice == nil || v.value.Cmp(choice) < 0) {
				choice = new(big.Rat).Set(v.value)
			}
		}
		if choice == nil {
			return measured{}, fmt.Errorf("RULE_NO_STANDARD_SIZE")
		}
		out.value = choice
	default:
		return fail()
	}
	return out, nil
}

// EvaluateRule has no registry/write capability. Only the service may execute
// approved versions for production calculations; this entry also validates cases.
func EvaluateRule(rule CalculationRule, inputs []NamedValue) ([]NamedValue, error) {
	values := map[string]measured{}
	given := map[string]TypedValue{}
	for _, v := range inputs {
		if _, ok := given[v.Name]; ok {
			return nil, fmt.Errorf("RULE_DUPLICATE_INPUT")
		}
		given[v.Name] = v.Value
	}
	declared := map[string]bool{}
	for _, spec := range rule.Inputs {
		if declared[spec.Name] || spec.Name == "" || spec.ValueType != "decimal" || spec.Unit == "" {
			return nil, fmt.Errorf("RULE_IO_INVALID")
		}
		declared[spec.Name] = true
		v, ok := given[spec.Name]
		if !ok {
			if spec.Required {
				return nil, fmt.Errorf("RULE_INPUT_MISSING")
			}
			continue
		}
		if v.Validate() != nil || v.Kind != "decimal" {
			return nil, fmt.Errorf("RULE_INPUT_TYPE")
		}
		n, err := measure(Quantity{Value: *v.Decimal, Unit: *v.Unit})
		dim, _ := unitScale(spec.Unit)
		if err != nil || n.dimension != dim {
			return nil, fmt.Errorf("RULE_UNIT_MISMATCH")
		}
		values[spec.Name] = n
	}
	for k := range given {
		if !declared[k] {
			return nil, fmt.Errorf("RULE_UNKNOWN_INPUT")
		}
	}
	expressions := map[string]RuleExpression{}
	if len(rule.Outputs) == 1 && rule.Expression.Operator != "outputs" {
		expressions[rule.Outputs[0].Name] = rule.Expression
	} else {
		if rule.Expression.Operator != "outputs" || rule.Expression.Input != nil || rule.Expression.Literal != nil {
			return nil, fmt.Errorf("RULE_OUTPUT_BINDINGS_REQUIRED")
		}
		for _, binding := range rule.Expression.Arguments {
			if binding.Operator != "output" || binding.Input == nil || binding.Literal != nil || len(binding.Arguments) != 1 {
				return nil, fmt.Errorf("RULE_OUTPUT_BINDINGS_INVALID")
			}
			if _, ok := expressions[*binding.Input]; ok {
				return nil, fmt.Errorf("RULE_DUPLICATE_OUTPUT")
			}
			expressions[*binding.Input] = binding.Arguments[0]
		}
	}
	if len(expressions) != len(rule.Outputs) || len(rule.Outputs) == 0 {
		return nil, fmt.Errorf("RULE_OUTPUT_BINDINGS_REQUIRED")
	}
	outputs := []NamedValue{}
	used := map[string]bool{}
	for _, spec := range rule.Outputs {
		if used[spec.Name] || spec.ValueType != "decimal" || spec.Unit == "" {
			return nil, fmt.Errorf("RULE_IO_INVALID")
		}
		used[spec.Name] = true
		e, ok := expressions[spec.Name]
		if !ok {
			return nil, fmt.Errorf("RULE_OUTPUT_BINDINGS_REQUIRED")
		}
		v, err := evaluate(e, values, 0)
		if err != nil {
			return nil, err
		}
		result, err := resultValue(v, spec.Unit)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, NamedValue{Name: spec.Name, Value: result})
	}
	return outputs, nil
}

func ValidateRule(rule CalculationRule) error {
	if rule.ID == "" || rule.Version < 1 || rule.ScopeID == "" || len(rule.ValidationCases) == 0 {
		return fmt.Errorf("RULE_METADATA_OR_CASES_REQUIRED")
	}
	seen := map[string]bool{}
	for _, test := range rule.ValidationCases {
		if test.ID == "" || seen[test.ID] {
			return fmt.Errorf("RULE_CASE_ID_INVALID")
		}
		seen[test.ID] = true
		outputs, err := EvaluateRule(rule, test.Inputs)
		if err != nil {
			return err
		}
		tolerance, ok := new(big.Rat).SetString(test.Tolerance)
		if !ok || !decimal(test.Tolerance) || tolerance.Sign() < 0 {
			return fmt.Errorf("RULE_TOLERANCE_REQUIRED")
		}
		expected := map[string]TypedValue{}
		for _, v := range test.Expected {
			if _, ok := expected[v.Name]; ok {
				return fmt.Errorf("RULE_CASE_OUTPUT_INVALID")
			}
			expected[v.Name] = v.Value
		}
		if len(outputs) != len(expected) {
			return fmt.Errorf("RULE_CASE_OUTPUT_INVALID")
		}
		for _, actual := range outputs {
			want, ok := expected[actual.Name]
			if !ok || want.Validate() != nil || want.Kind != "decimal" {
				return fmt.Errorf("RULE_CASE_OUTPUT_INVALID")
			}
			a, _ := measure(Quantity{*actual.Value.Decimal, *actual.Value.Unit})
			b, _ := measure(Quantity{*want.Decimal, *want.Unit})
			_, scale := unitScale(*want.Unit)
			delta := new(big.Rat).Abs(new(big.Rat).Sub(a.value, b.value))
			if a.dimension != b.dimension || delta.Cmp(new(big.Rat).Mul(tolerance, scale)) > 0 {
				return fmt.Errorf("RULE_CASE_MISMATCH")
			}
		}
	}
	return nil
}

func RunRule(rule CalculationRule, project, revision string, inputs []NamedValue) (CalculationRun, error) {
	now := time.Now().UTC()
	ref := VersionReference{ID: rule.ID, Version: rule.Version}
	run := CalculationRun{ID: uuid.NewString(), ProjectID: project, RevisionID: revision, ModuleInstanceID: string(rule.Domain), ProviderID: "engineering-expression", ProviderVersion: "1", Rule: &ref, InputDigest: Digest([]any{rule.ID, rule.Version, inputs}), Status: "failed", Compatibility: "compatible", Inputs: inputs, Outputs: []NamedValue{}, Formulas: []FormulaEvidence{}, Errors: []CalculationError{}, StartedAt: now}
	var err error
	if rule.ApprovalStatus != "approved" {
		err = fmt.Errorf("RULE_APPROVAL_REQUIRED")
	} else if rule.ScopeType == "project" && rule.ScopeID != project {
		err = fmt.Errorf("RULE_SCOPE_MISMATCH")
	} else {
		run.Outputs, err = EvaluateRule(rule, inputs)
	}
	if err == nil {
		run.Status = "succeeded"
	} else {
		run.Errors = append(run.Errors, CalculationError{Code: err.Error(), Message: err.Error()})
	}
	done := time.Now().UTC()
	run.FinishedAt = &done
	return run, err
}

// EqualQuantities compares exact values after explicit unit normalization.
func EqualQuantities(a, b Quantity) bool {
	x, ex := measure(a)
	y, ey := measure(b)
	return ex == nil && ey == nil && x.dimension == y.dimension && x.value.Cmp(y.value) == 0
}
