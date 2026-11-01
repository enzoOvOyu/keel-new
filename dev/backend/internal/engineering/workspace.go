package engineering

import "time"

const EquipmentTarget = "CMDB_PLANNED_EQUIPMENT"
const FacilityTarget = "CMDB_PLANNED_FACILITY"
const PlacementTarget = "CMDB_PLANNED_PLACEMENT"
const ConnectivityTarget = "CMDB_PLANNED_CONNECTIVITY"
const CablingTarget = "ENGINEERING_CABLING"

var PhysicalTargets = []string{EquipmentTarget, FacilityTarget, PlacementTarget}
var PublishTargets = []string{EquipmentTarget, FacilityTarget, PlacementTarget, ConnectivityTarget, CablingTarget, PowerTarget, MaterialTarget, WorkloadTarget}

func TargetDependencies(target string) []string {
	switch target {
	case PlacementTarget:
		return []string{EquipmentTarget, FacilityTarget}
	case ConnectivityTarget:
		return []string{EquipmentTarget}
	case CablingTarget:
		return []string{ConnectivityTarget}
	}
	return nil
}

type ConnectivityEdit struct {
	ExpectedRevision int                `json:"expected_revision"`
	Port             *PlannedPort       `json:"port,omitempty"`
	Connection       *PlannedConnection `json:"connection,omitempty"`
	Cable            *CablePlan         `json:"cable,omitempty"`
	Optic            *OpticDemand       `json:"optic,omitempty"`
	Reason           string             `json:"reason"`
}

type ColumnBinding struct {
	Field  string `json:"field"`
	Column int    `json:"column"`
	Unit   string `json:"unit"`
}

type CalculationRequest struct {
	Operation        string            `json:"operation,omitempty"`
	ExpectedRevision int               `json:"expected_revision"`
	Rule             *VersionReference `json:"rule,omitempty"`
	RequestedCells   []FormulaCell     `json:"requested_cells"`
	Inputs           []NamedValue      `json:"inputs"`
}
type CalculationPage struct {
	Run           CalculationRun `json:"run"`
	TotalFormulas int            `json:"total_formulas"`
	Offset        int            `json:"offset"`
	Limit         int            `json:"limit"`
	Stale         bool           `json:"stale"`
}
type RuleDraft struct {
	ExpectedVersion int             `json:"expected_version"`
	Rule            CalculationRule `json:"rule"`
	Reason          string          `json:"reason"`
}
type RuleReview struct {
	ExpectedDigest string `json:"expected_digest"`
	Decision       string `json:"decision"`
	Reason         string `json:"reason"`
}
type RuleView struct {
	Rule   CalculationRule `json:"rule"`
	Digest string          `json:"digest"`
}
type ConstantBinding struct {
	Field         string   `json:"field"`
	Value         string   `json:"value"`
	EvidenceCells []string `json:"evidence_cells"`
}
type RegionMapping struct {
	Sheet      string            `json:"sheet"`
	Bounds     Bounds            `json:"bounds"`
	Kind       string            `json:"kind"`
	Semantic   string            `json:"semantic"`
	HeaderRows []int             `json:"header_rows"`
	Columns    []ColumnBinding   `json:"columns"`
	Constants  []ConstantBinding `json:"constants"`
}
type ModuleDetection struct {
	ModuleID   string   `json:"module_id"`
	Applicable bool     `json:"applicable"`
	Reason     string   `json:"reason"`
	Sheets     []string `json:"sheets"`
}
type EngineeringProposal struct {
	Summary   string            `json:"summary"`
	Regions   []RegionMapping   `json:"regions"`
	Modules   []ModuleDetection `json:"modules"`
	Questions []string          `json:"questions"`
}
type SourceEvidence struct {
	ID               ID     `json:"id"`
	Sheet            string `json:"sheet"`
	Cell             string `json:"cell"`
	Field            string `json:"field"`
	EntityID         ID     `json:"entity_id"`
	Formula          bool   `json:"formula"`
	CalculationRunID *ID    `json:"calculation_run_id,omitempty"`
}
type TargetStatus struct {
	Target        string      `json:"target"`
	State         ModuleState `json:"state"`
	RevisionID    ID          `json:"revision_id"`
	Digest        string      `json:"digest"`
	ReviewID      *ID         `json:"review_id,omitempty"`
	PublicationID *ID         `json:"publication_id,omitempty"`
	ErrorCode     *string     `json:"error_code,omitempty"`
}
type EngineeringJob struct {
	ID        ID        `json:"id"`
	State     string    `json:"state"`
	ErrorCode *string   `json:"error_code,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type EngineeringStep struct {
	Sequence int       `json:"sequence"`
	Tool     string    `json:"tool"`
	Detail   string    `json:"detail"`
	State    string    `json:"state"`
	At       time.Time `json:"at"`
}
type Workspace struct {
	ID             ID                `json:"id"`
	ProjectID      ID                `json:"project_id"`
	ImportID       ID                `json:"import_id"`
	Filename       string            `json:"filename"`
	SHA256         string            `json:"sha256"`
	Revision       int               `json:"revision"`
	HeadRevisionID *ID               `json:"head_revision_id,omitempty"`
	Summary        string            `json:"summary"`
	TotalCells     int               `json:"total_cells"`
	MappedCells    int               `json:"mapped_cells"`
	RetainedCells  int               `json:"retained_cells"`
	Modules        []ModuleDetection `json:"modules"`
	Targets        []TargetStatus    `json:"targets"`
	Issues         []ValidationIssue `json:"issues"`
	Questions      []string          `json:"questions"`
	LatestJob      *EngineeringJob   `json:"latest_job,omitempty"`
	Steps          []EngineeringStep `json:"steps"`
	CreatedAt      time.Time         `json:"created_at"`
}
type RevisionEntry struct {
	ID        ID        `json:"id"`
	Sequence  int       `json:"sequence"`
	Actor     string    `json:"actor"`
	Origin    string    `json:"origin"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}
type RevisionDifference struct {
	EntityID ID     `json:"entity_id"`
	Kind     string `json:"kind"`
	Change   string `json:"change"`
	Before   string `json:"before"`
	After    string `json:"after"`
}
type ModelEdit struct {
	ExpectedRevision int           `json:"expected_revision"`
	Model            PhysicalModel `json:"model"`
	Reason           string        `json:"reason"`
}
type TargetAction struct {
	ExpectedRevision int      `json:"expected_revision"`
	Targets          []string `json:"targets"`
	Decision         string   `json:"decision"`
	Reason           string   `json:"reason"`
}

const PowerTarget = "ENGINEERING_POWER"

type PowerEdit struct {
	ExpectedRevision int       `json:"expected_revision"`
	Plan             PowerPlan `json:"plan"`
	Reason           string    `json:"reason"`
}
type ResultBinding struct {
	EntityID   string `json:"entity_id"`
	Field      string `json:"field"`
	OutputName string `json:"output_name"`
}
type CalculationApplication struct {
	ExpectedRevision int             `json:"expected_revision"`
	RunID            string          `json:"run_id"`
	Bindings         []ResultBinding `json:"bindings"`
	Reason           string          `json:"reason"`
}

const MaterialTarget = "ENGINEERING_MATERIAL"
const WorkloadTarget = "ENGINEERING_WORKLOAD"

type EstimateEdit struct {
	ExpectedRevision int               `json:"expected_revision"`
	Material         *MaterialEstimate `json:"material,omitempty"`
	Workload         *WorkloadEstimate `json:"workload,omitempty"`
	Reason           string            `json:"reason"`
}
