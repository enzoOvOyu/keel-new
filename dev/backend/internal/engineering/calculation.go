package engineering

import (
	"context"
	"io"
	"time"
)

type RuleIO struct {
	Name      string `json:"name"`
	ValueType string `json:"value_type"`
	Unit      string `json:"unit"`
	Required  bool   `json:"required"`
}

// Expression is data, not Python/JavaScript source. A future executor must
// explicitly support each operator and check units before evaluating it.
type RuleExpression struct {
	Operator  string           `json:"operator"`
	Input     *string          `json:"input,omitempty"`
	Literal   *Quantity        `json:"literal,omitempty"`
	Arguments []RuleExpression `json:"arguments"`
}
type NamedValue struct {
	Name  string     `json:"name"`
	Value TypedValue `json:"value"`
}
type RuleCase struct {
	ID        ID           `json:"id"`
	Inputs    []NamedValue `json:"inputs"`
	Expected  []NamedValue `json:"expected"`
	Tolerance string       `json:"tolerance"`
}
type CalculationRule struct {
	ID               ID                 `json:"id"`
	Version          int                `json:"version"`
	Domain           Domain             `json:"domain"`
	ScopeType        string             `json:"scope_type"`
	ScopeID          ID                 `json:"scope_id"`
	Inputs           []RuleIO           `json:"inputs"`
	Outputs          []RuleIO           `json:"outputs"`
	Expression       RuleExpression     `json:"expression"`
	Dependencies     []VersionReference `json:"dependencies"`
	ValidationCases  []RuleCase         `json:"validation_cases"`
	ApprovalStatus   string             `json:"approval_status"`
	ReviewDecisionID *ID                `json:"review_decision_id,omitempty"`
}
type FormulaCell struct {
	SheetID ID     `json:"sheet_id"`
	Address string `json:"address"`
}
type FormulaDependency struct {
	From      FormulaCell `json:"from"`
	ToSheetID ID          `json:"to_sheet_id"`
	ToRange   string      `json:"to_range"`
	Kind      string      `json:"kind"`
}
type FormulaEvidence struct {
	Cell              FormulaCell         `json:"cell"`
	Expression        string              `json:"expression"`
	CachedValue       *TypedValue         `json:"cached_value,omitempty"`
	RecalculatedValue *TypedValue         `json:"recalculated_value,omitempty"`
	Dependencies      []FormulaDependency `json:"dependencies"`
	DependencyStatus  string              `json:"dependency_status"`
	Compatibility     string              `json:"compatibility"`
	ErrorCode         *string             `json:"error_code,omitempty"`
}
type CalculationError struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Cell    *FormulaCell `json:"cell,omitempty"`
}
type CalculationRun struct {
	ID               ID                 `json:"id"`
	ProjectID        ID                 `json:"project_id"`
	ModuleInstanceID ID                 `json:"module_instance_id"`
	RevisionID       ID                 `json:"revision_id"`
	ProviderID       string             `json:"provider_id"`
	ProviderVersion  string             `json:"provider_version"`
	Rule             *VersionReference  `json:"rule,omitempty"`
	SourceSHA256     string             `json:"source_sha256"`
	InputDigest      string             `json:"input_digest"`
	Status           string             `json:"status"`
	Compatibility    string             `json:"compatibility"`
	Inputs           []NamedValue       `json:"inputs"`
	Outputs          []NamedValue       `json:"outputs"`
	Formulas         []FormulaEvidence  `json:"formulas"`
	Errors           []CalculationError `json:"errors"`
	StartedAt        time.Time          `json:"started_at"`
	FinishedAt       *time.Time         `json:"finished_at,omitempty"`
}
type CalculationCapabilities struct {
	ID                    string   `json:"id"`
	Version               string   `json:"version"`
	Available             bool     `json:"available"`
	Formulas              []string `json:"formulas"`
	SupportsNamedRanges   bool     `json:"supports_named_ranges"`
	SupportsExternalLinks bool     `json:"supports_external_links"`
}
type SpreadsheetCalculationRequest struct {
	Source WorkbookSource
	// OpenSource must return a fresh read-only stream. Providers write output only
	// to isolated storage; the interface intentionally has no source mutation API.
	OpenSource     func(context.Context) (io.ReadCloser, error)
	RequestedCells []FormulaCell
	Parameters     []NamedValue
	Revision       ModelRevision
}

// No provider is silently substituted. Unsupported calculation is not success
// and an existing Excel cache is never a recalculated result.
type SpreadsheetCalculationProvider interface {
	Capabilities(context.Context) (CalculationCapabilities, error)
	Calculate(context.Context, SpreadsheetCalculationRequest) (CalculationRun, error)
}
