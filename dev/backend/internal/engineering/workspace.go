package engineering

import "time"

const EquipmentTarget = "CMDB_PLANNED_EQUIPMENT"
const FacilityTarget = "CMDB_PLANNED_FACILITY"
const PlacementTarget = "CMDB_PLANNED_PLACEMENT"

var PhysicalTargets = []string{EquipmentTarget, FacilityTarget, PlacementTarget}

type ColumnBinding struct {
	Field  string `json:"field"`
	Column int    `json:"column"`
	Unit   string `json:"unit"`
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
	ID       ID     `json:"id"`
	Sheet    string `json:"sheet"`
	Cell     string `json:"cell"`
	Field    string `json:"field"`
	EntityID ID     `json:"entity_id"`
	Formula  bool   `json:"formula"`
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
