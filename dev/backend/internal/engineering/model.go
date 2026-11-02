// Package engineering owns proposed engineering models. It has no database,
// CMDB, approval identity or network execution credentials.
package engineering

import "time"

type ID = string
type Domain string

const (
	SourceDomain       Domain = "source"
	EquipmentDomain    Domain = "equipment"
	FacilityDomain     Domain = "facility_placement"
	PowerDomain        Domain = "power_cooling"
	ConnectivityDomain Domain = "connectivity_cabling"
	MaterialDomain     Domain = "material_workload"
	NetworkDomain      Domain = "network_design"
	ReleaseDomain      Domain = "validation_release"
)

type Maturity string

const (
	Estimated  Maturity = "Estimated"
	Calculated Maturity = "Calculated"
	Confirmed  Maturity = "Confirmed"
	AsBuilt    Maturity = "As-built"
)

// Quantity uses a decimal string so procurement quantities and calculation inputs
// do not lose precision during JSON transport. Unit is mandatory, including count.
type Quantity struct {
	Value string `json:"value"`
	Unit  string `json:"unit"`
}
type Bounds struct {
	StartRow    int `json:"start_row"`
	EndRow      int `json:"end_row"`
	StartColumn int `json:"start_column"`
	EndColumn   int `json:"end_column"`
}
type WorkbookSource struct {
	ID            ID        `json:"id"`
	ProjectID     ID        `json:"project_id"`
	ImportID      ID        `json:"import_id"`
	DocumentID    ID        `json:"document_id"`
	SHA256        string    `json:"sha256"`
	Filename      string    `json:"filename"`
	ByteSize      int64     `json:"byte_size"`
	ParserVersion string    `json:"parser_version"`
	CreatedAt     time.Time `json:"created_at"`
}
type HeaderCell struct {
	Row    int     `json:"row"`
	Column int     `json:"column"`
	Level  int     `json:"level"`
	Parent *string `json:"parent,omitempty"`
}
type WorkbookRegion struct {
	ID            ID           `json:"id"`
	SourceID      ID           `json:"source_id"`
	SheetID       ID           `json:"sheet_id"`
	Bounds        Bounds       `json:"bounds"`
	Kind          string       `json:"kind"`
	SemanticLabel string       `json:"semantic_label"`
	Headers       []HeaderCell `json:"headers"`
	Merges        []Bounds     `json:"merges"`
}

// Exactly one value slot must match Kind. Extensions use a versioned definition,
// never an untyped map that could override core entity fields.
type TypedValue struct {
	Kind      string  `json:"kind"`
	Text      *string `json:"text,omitempty"`
	Decimal   *string `json:"decimal,omitempty"`
	Boolean   *bool   `json:"boolean,omitempty"`
	Reference *ID     `json:"reference,omitempty"`
	Unit      *string `json:"unit,omitempty"`
}
type AttributeDefinition struct {
	ID             ID       `json:"id"`
	Version        int      `json:"version"`
	EntityKind     string   `json:"entity_kind"`
	Key            string   `json:"key"`
	ValueType      string   `json:"value_type"`
	Unit           *string  `json:"unit,omitempty"`
	EnumValues     []string `json:"enum_values"`
	Required       bool     `json:"required"`
	ApprovalStatus string   `json:"approval_status"`
}
type AttributeValue struct {
	DefinitionID      ID         `json:"definition_id"`
	DefinitionVersion int        `json:"definition_version"`
	Value             TypedValue `json:"value"`
}
type SemanticFact struct {
	ID           ID         `json:"id"`
	ProjectID    ID         `json:"project_id"`
	RevisionID   ID         `json:"revision_id"`
	RegionID     ID         `json:"region_id"`
	SubjectID    *ID        `json:"subject_id,omitempty"`
	Predicate    string     `json:"predicate"`
	Value        TypedValue `json:"value"`
	ClaimStatus  string     `json:"claim_status"`
	Maturity     Maturity   `json:"maturity"`
	ProvenanceID ID         `json:"provenance_id"`
}
type EvidenceReference struct {
	ID               ID     `json:"id"`
	SourceID         ID     `json:"source_id"`
	SheetID          ID     `json:"sheet_id"`
	Bounds           Bounds `json:"bounds"`
	Kind             string `json:"kind"`
	SourceSHA256     string `json:"source_sha256"`
	CalculationRunID *ID    `json:"calculation_run_id,omitempty"`
}
type ProvenanceInput struct {
	Kind       string `json:"kind"`
	ID         ID     `json:"id"`
	RevisionID *ID    `json:"revision_id,omitempty"`
}
type Provenance struct {
	ID              ID                `json:"id"`
	ProducerType    string            `json:"producer_type"`
	ProducerID      string            `json:"producer_id"`
	ProducerVersion string            `json:"producer_version"`
	RunID           ID                `json:"run_id"`
	ActorID         *ID               `json:"actor_id,omitempty"`
	Inputs          []ProvenanceInput `json:"inputs"`
	CreatedAt       time.Time         `json:"created_at"`
}
type CanonicalEntity struct {
	ID          ID     `json:"id"`
	ProjectID   ID     `json:"project_id"`
	Domain      Domain `json:"domain"`
	Kind        string `json:"kind"`
	PlanningKey string `json:"planning_key"`
	BOMItemID   *ID    `json:"bom_item_id,omitempty"`
	CMDBAssetID *ID    `json:"cmdb_asset_id,omitempty"`
}
type EntityVersion struct {
	EntityID     ID               `json:"entity_id"`
	RevisionID   ID               `json:"revision_id"`
	EvidenceIDs  []ID             `json:"evidence_ids"`
	ProvenanceID ID               `json:"provenance_id"`
	Maturity     Maturity         `json:"maturity"`
	Attributes   []AttributeValue `json:"attributes"`
}
type EquipmentPlan struct {
	EntityVersion
	EquipmentType    string  `json:"equipment_type"`
	ModelRef         *ID     `json:"model_ref,omitempty"`
	SourceModelLabel string  `json:"source_model_label"`
	Quantity         int64   `json:"quantity"`
	IdentityKind     string  `json:"identity_kind"`
	Label            string  `json:"label"`
	Serial           *string `json:"serial,omitempty"`
	HeightU          *int    `json:"height_u,omitempty"`
}
type DeviceAssembly struct {
	EntityVersion
	ParentID     ID      `json:"parent_id"`
	ChildID      ID      `json:"child_id"`
	RelationType string  `json:"relation_type"`
	Quantity     int64   `json:"quantity"`
	SlotLabel    *string `json:"slot_label,omitempty"`
}
type FacilityModel struct {
	EntityVersion
	Kind               string  `json:"kind"`
	Code               string  `json:"code,omitempty"`
	Label              string  `json:"label"`
	ParentID           *ID     `json:"parent_id,omitempty"`
	CapacityU          *int    `json:"capacity_u,omitempty"`
	WidthMM            *string `json:"width_mm,omitempty"`
	HeightMM           *string `json:"height_mm,omitempty"`
	DepthMM            *string `json:"depth_mm,omitempty"`
	XMM                *string `json:"x_mm,omitempty"`
	YMM                *string `json:"y_mm,omitempty"`
	ZMM                *string `json:"z_mm,omitempty"`
	OrientationDegrees *string `json:"orientation_degrees,omitempty"`
	CoordinateFrameID  *ID     `json:"coordinate_frame_id,omitempty"`
}
type RackPlacement struct {
	EntityVersion
	EquipmentID      ID      `json:"equipment_id"`
	RackID           ID      `json:"rack_id"`
	StartU           *int    `json:"start_u,omitempty"`
	HeightU          *int    `json:"height_u,omitempty"`
	MountFace        string  `json:"mount_face"`
	InstallationType string  `json:"installation_type,omitempty"`
	PositionCode     string  `json:"position_code,omitempty"`
	DepthMM          *string `json:"depth_mm,omitempty"`
}
type EquipmentPowerProfile struct {
	EquipmentID ID       `json:"equipment_id"`
	Basis       string   `json:"basis"`
	Power       Quantity `json:"power"`
	CoolingType string   `json:"cooling_type"`
	EvidenceIDs []ID     `json:"evidence_ids"`
}
type RackPowerPlan struct {
	CapacityBasis    string    `json:"capacity_basis,omitempty"`
	RackID           ID        `json:"rack_id"`
	Capacity         *Quantity `json:"capacity,omitempty"`
	CoolingCapacity  *Quantity `json:"cooling_capacity,omitempty"`
	CoolingType      string    `json:"cooling_type"`
	PlannedLoad      *Quantity `json:"planned_load,omitempty"`
	Remaining        *Quantity `json:"remaining,omitempty"`
	CalculationRunID *ID       `json:"calculation_run_id,omitempty"`
}
type PowerPlan struct {
	EntityVersion
	Scope             *PowerScope             `json:"scope,omitempty"`
	Circuits          []PowerCircuit          `json:"circuits,omitempty"`
	Supplies          []EquipmentSupply       `json:"supplies,omitempty"`
	EquipmentProfiles []EquipmentPowerProfile `json:"equipment_profiles"`
	Racks             []RackPowerPlan         `json:"racks"`
}
type PlannedPort struct {
	EntityVersion
	EquipmentID   ID      `json:"equipment_id"`
	Name          string  `json:"name"`
	MAC           *string `json:"mac,omitempty"`
	Connector     string  `json:"connector,omitempty"`
	ParentPortID  *ID     `json:"parent_port_id,omitempty"`
	BreakoutIndex *int    `json:"breakout_index,omitempty"`
	Role          string  `json:"role"`
	RateBPS       *int64  `json:"rate_bps,omitempty"`
	Medium        string  `json:"medium"`
}
type PlannedConnection struct {
	EntityVersion
	APortID         ID     `json:"a_port_id"`
	ZPortID         ID     `json:"z_port_id"`
	PatchPortIDs    []ID   `json:"patch_port_ids"`
	TerminationType string `json:"termination_type,omitempty"`
	BRackPosition   string `json:"b_rack_position,omitempty"`
}
type ConnectivityPlan struct {
	EntityVersion
	Ports       []PlannedPort       `json:"ports"`
	Connections []PlannedConnection `json:"connections"`
}
type CableSpec struct {
	ID               ID     `json:"id"`
	Medium           string `json:"medium"`
	ConnectorA       string `json:"connector_a"`
	ConnectorZ       string `json:"connector_z"`
	RateBPS          *int64 `json:"rate_bps,omitempty"`
	SourceModelLabel string `json:"source_model_label"`
}
type RouteSegment struct {
	FromID           ID        `json:"from_id"`
	ToID             ID        `json:"to_id"`
	Distance         *Quantity `json:"distance,omitempty"`
	CalculationRunID *ID       `json:"calculation_run_id,omitempty"`
}
type LengthEvidence struct {
	Length           Quantity `json:"length"`
	EvidenceIDs      []ID     `json:"evidence_ids"`
	CalculationRunID *ID      `json:"calculation_run_id,omitempty"`
}
type CablePlan struct {
	EntityVersion
	SegmentIndex  *int              `json:"segment_index,omitempty"`
	Label         *string           `json:"label,omitempty"`
	ConnectionID  ID                `json:"connection_id"`
	Spec          CableSpec         `json:"spec"`
	Route         []RouteSegment    `json:"route"`
	RuleVersion   *VersionReference `json:"rule_version,omitempty"`
	Estimated     *LengthEvidence   `json:"estimated,omitempty"`
	Calculated    *LengthEvidence   `json:"calculated,omitempty"`
	SelectedStock *LengthEvidence   `json:"selected_stock,omitempty"`
	Confirmed     *LengthEvidence   `json:"confirmed,omitempty"`
	AsBuilt       *LengthEvidence   `json:"as_built,omitempty"`
}

type OpticDemand struct {
	EntityVersion
	PhysicalPortID   ID     `json:"physical_port_id"`
	SourceModelLabel string `json:"source_model_label"`
	Quantity         int64  `json:"quantity"`
}
type MaterialEstimate struct {
	EntityVersion
	SourceModelLabel         string    `json:"source_model_label"`
	RequiredCalculationRunID *ID       `json:"required_calculation_run_id,omitempty"`
	SpecRef                  ID        `json:"spec_ref"`
	Required                 Quantity  `json:"required"`
	Purchased                *Quantity `json:"purchased,omitempty"`
	Spares                   *Quantity `json:"spares,omitempty"`
	PurchaseGap              *Quantity `json:"purchase_gap,omitempty"`
	CalculationRunID         *ID       `json:"calculation_run_id,omitempty"`
}
type WorkloadEstimate struct {
	EntityVersion
	SourceModelLabel         string    `json:"source_model_label"`
	QuantityCalculationRunID *ID       `json:"quantity_calculation_run_id,omitempty"`
	WorkType                 string    `json:"work_type"`
	SpecRef                  *ID       `json:"spec_ref,omitempty"`
	Quantity                 Quantity  `json:"quantity"`
	StandardLength           *Quantity `json:"standard_length,omitempty"`
	LaborHours               *Quantity `json:"labor_hours,omitempty"`
	CalculationRunID         *ID       `json:"calculation_run_id,omitempty"`
}
type NetworkAllocation struct {
	VRFRef       *ID     `json:"vrf_ref,omitempty"`
	ID           ID      `json:"id"`
	Kind         string  `json:"kind"`
	InterfaceID  *ID     `json:"interface_id,omitempty"`
	EntityID     ID      `json:"entity_id"`
	Address      *string `json:"address,omitempty"`
	PrefixLength *int    `json:"prefix_length,omitempty"`
	ASN          *int64  `json:"asn,omitempty"`
	EvidenceIDs  []ID    `json:"evidence_ids"`
}
type NetworkFabric struct {
	ID          ID     `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	EvidenceIDs []ID   `json:"evidence_ids"`
}
type NetworkPlane struct {
	ID          ID     `json:"id"`
	FabricID    ID     `json:"fabric_id"`
	Name        string `json:"name"`
	EvidenceIDs []ID   `json:"evidence_ids"`
	Profile     string `json:"profile,omitempty"`
}
type DeviceRole struct {
	EquipmentID ID     `json:"equipment_id"`
	PlaneID     ID     `json:"plane_id"`
	Role        string `json:"role"`
	EvidenceIDs []ID   `json:"evidence_ids"`
}
type InterfaceMapping struct {
	ID             ID     `json:"id"`
	PhysicalPortID ID     `json:"physical_port_id"`
	PlaneID        ID     `json:"plane_id"`
	LogicalName    string `json:"logical_name"`
	EvidenceIDs    []ID   `json:"evidence_ids"`
}
type BGPPlan struct {
	ID                   ID     `json:"id"`
	AInterfaceID         ID     `json:"a_interface_id"`
	ZInterfaceID         ID     `json:"z_interface_id"`
	LocalASNAllocationID ID     `json:"local_asn_allocation_id"`
	PeerASNAllocationID  ID     `json:"peer_asn_allocation_id"`
	AFI                  string `json:"afi"`
	SAFI                 string `json:"safi"`
	PolicyRef            *ID    `json:"policy_ref,omitempty"`
	EvidenceIDs          []ID   `json:"evidence_ids"`
}
type BFDPlan struct {
	SessionID         ID   `json:"session_id"`
	MinTXMilliseconds int  `json:"min_tx_milliseconds"`
	MinRXMilliseconds int  `json:"min_rx_milliseconds"`
	Multiplier        int  `json:"multiplier"`
	EvidenceIDs       []ID `json:"evidence_ids"`
}
type QoSPlan struct {
	ID                 ID    `json:"id"`
	InterfaceIDs       []ID  `json:"interface_ids"`
	RoCEVersion        *int  `json:"roce_version,omitempty"`
	LosslessPriorities []int `json:"lossless_priorities"`
	ECNEnabled         *bool `json:"ecn_enabled,omitempty"`
	PFCEnabled         *bool `json:"pfc_enabled,omitempty"`
	PolicyRef          *ID   `json:"policy_ref,omitempty"`
	EvidenceIDs        []ID  `json:"evidence_ids"`
}
type AdaptiveRoutingPlan struct {
	FabricID    ID   `json:"fabric_id"`
	Enabled     bool `json:"enabled"`
	PolicyRef   ID   `json:"policy_ref"`
	EvidenceIDs []ID `json:"evidence_ids"`
}
type EVPNVXLANPlan struct {
	ID                 ID       `json:"id"`
	FabricID           ID       `json:"fabric_id"`
	VNI                int      `json:"vni"`
	VLAN               *int     `json:"vlan,omitempty"`
	VRFRef             *ID      `json:"vrf_ref,omitempty"`
	VTEPAllocationIDs  []ID     `json:"vtep_allocation_ids"`
	RouteDistinguisher string   `json:"route_distinguisher"`
	ImportTargets      []string `json:"import_targets"`
	ExportTargets      []string `json:"export_targets"`
	EvidenceIDs        []ID     `json:"evidence_ids"`
}
type NetworkPlan struct {
	EntityVersion
	PhysicalRevisionID ID                    `json:"physical_revision_id"`
	SourceIDs          []ID                  `json:"source_ids"`
	Fabrics            []NetworkFabric       `json:"fabrics"`
	Planes             []NetworkPlane        `json:"planes"`
	DeviceRoles        []DeviceRole          `json:"device_roles"`
	InterfaceMappings  []InterfaceMapping    `json:"interface_mappings"`
	Allocations        []NetworkAllocation   `json:"allocations"`
	BGP                []BGPPlan             `json:"bgp"`
	BFD                []BFDPlan             `json:"bfd"`
	QoS                []QoSPlan             `json:"qos"`
	AdaptiveRouting    []AdaptiveRoutingPlan `json:"adaptive_routing"`
	EVPNVXLAN          []EVPNVXLANPlan       `json:"evpn_vxlan"`
}
type VersionReference struct {
	ID      ID  `json:"id"`
	Version int `json:"version"`
}
type RevisionInput struct {
	Kind       string `json:"kind"`
	ID         ID     `json:"id"`
	RevisionID ID     `json:"revision_id"`
	Digest     string `json:"digest"`
}
type ModelRevision struct {
	ID               ID              `json:"id"`
	ProjectID        ID              `json:"project_id"`
	ModuleInstanceID ID              `json:"module_instance_id"`
	Sequence         int             `json:"sequence"`
	ParentID         *ID             `json:"parent_id,omitempty"`
	InputDigest      string          `json:"input_digest"`
	ContentDigest    string          `json:"content_digest"`
	AuthorType       string          `json:"author_type"`
	AuthorID         ID              `json:"author_id"`
	Inputs           []RevisionInput `json:"inputs"`
	CreatedAt        time.Time       `json:"created_at"`
}
type ValidationIssue struct {
	ID            ID                `json:"id"`
	ProjectID     ID                `json:"project_id"`
	RevisionID    ID                `json:"revision_id"`
	Resolution    string            `json:"resolution"`
	RuleVersion   *VersionReference `json:"rule_version,omitempty"`
	Code          string            `json:"code"`
	Severity      string            `json:"severity"`
	ModuleID      ID                `json:"module_id"`
	EntityID      ID                `json:"entity_id"`
	Field         string            `json:"field"`
	Message       string            `json:"message"`
	PublishTarget string            `json:"publish_target"`
	EvidenceIDs   []ID              `json:"evidence_ids"`
}
type ReviewDecision struct {
	ID               ID        `json:"id"`
	ProjectID        ID        `json:"project_id"`
	ModuleInstanceID ID        `json:"module_instance_id"`
	RevisionID       ID        `json:"revision_id"`
	Digest           string    `json:"digest"`
	Decision         string    `json:"decision"`
	ActorID          ID        `json:"actor_id"`
	Reason           string    `json:"reason"`
	CreatedAt        time.Time `json:"created_at"`
}
type PublishRecord struct {
	ID               ID         `json:"id"`
	ProjectID        ID         `json:"project_id"`
	ModuleInstanceID ID         `json:"module_instance_id"`
	RevisionID       ID         `json:"revision_id"`
	ReviewID         ID         `json:"review_id"`
	Target           string     `json:"target"`
	IdempotencyKey   string     `json:"idempotency_key"`
	Digest           string     `json:"digest"`
	State            string     `json:"state"`
	OutboxEventID    ID         `json:"outbox_event_id"`
	Receipt          *string    `json:"receipt,omitempty"`
	ErrorCode        *string    `json:"error_code,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	PublishedAt      *time.Time `json:"published_at,omitempty"`
}
