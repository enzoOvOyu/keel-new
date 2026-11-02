package engineering

// Modeling inputs are editable staging documents. Their adoption reuses the
// standard-input validators and versioned domain models; saving is not approval.
type ModelingInput struct {
	TableID      string         `json:"table_id"`
	Version      int            `json:"version"`
	BaseRevision int            `json:"base_revision"`
	State        string         `json:"state"`
	Rows         int            `json:"rows"`
	Issues       []InputProblem `json:"issues"`
	CSV          string         `json:"csv,omitempty"`
	Reason       string         `json:"reason"`
}
type ModelingInputs struct {
	Revision            int                          `json:"revision"`
	Items               []ModelingInput              `json:"items"`
	PlacementReferences []EndpointPlacementReference `json:"placement_references,omitempty"`
}

// Read projection of existing placement inputs/models, never a second placement
// fact source. Candidate references do not satisfy placement or cabling gates.
type EndpointPlacementReference struct {
	Serial       string `json:"serial"`
	State        string `json:"state"`
	EquipmentID  ID     `json:"equipment_id,omitempty"`
	PlacementID  ID     `json:"placement_id,omitempty"`
	RoomCode     string `json:"room_code,omitempty"`
	RackCode     string `json:"rack_code,omitempty"`
	StartU       *int   `json:"start_u,omitempty"`
	PositionCode string `json:"position_code,omitempty"`
	InputVersion int    `json:"input_version,omitempty"`
	InputRows    []int  `json:"input_rows"`
}
type ModelingInputEdit struct {
	ExpectedRevision int `json:"expected_revision"`
	InputVersion     int `json:"input_version"`
	InputPreviewRequest
	Reason string `json:"reason"`
}
type ModelingInputVersion struct {
	TableID string `json:"table_id"`
	Version int    `json:"version"`
}
type ModelingAdoption struct {
	ExpectedRevision int                    `json:"expected_revision"`
	Step             int                    `json:"step"`
	Inputs           []ModelingInputVersion `json:"inputs"`
	Reason           string                 `json:"reason"`
}
type KickoffPublication struct {
	Target        string `json:"target"`
	PublicationID string `json:"publication_id"`
	RevisionID    string `json:"revision_id"`
}
type KickoffHandoff struct {
	ID             string               `json:"id"`
	ProjectID      string               `json:"project_id"`
	SourceID       string               `json:"source_id"`
	Revision       int                  `json:"revision"`
	State          string               `json:"state"`
	Blockers       []string             `json:"blockers"`
	Publications   []KickoffPublication `json:"publications"`
	EquipmentIDs   []string             `json:"equipment_ids"`
	PortIDs        []string             `json:"port_ids"`
	NetworkPlanIDs []string             `json:"network_plan_ids"`
	NextStage      string               `json:"next_stage"`
	NextStageState string               `json:"next_stage_state"`
}
type KickoffHandoffRequest struct {
	ExpectedRevision int    `json:"expected_revision"`
	Reason           string `json:"reason"`
}
