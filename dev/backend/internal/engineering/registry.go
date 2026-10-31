package engineering

import (
	"fmt"
	"sort"
)

type ModuleState string

const (
	NotApplicable ModuleState = "NOT_APPLICABLE"
	Detected      ModuleState = "DETECTED"
	NeedsInput    ModuleState = "NEEDS_INPUT"
	Ready         ModuleState = "READY"
	Processing    ModuleState = "PROCESSING"
	InReview      ModuleState = "IN_REVIEW"
	Approved      ModuleState = "APPROVED"
	Published     ModuleState = "PUBLISHED"
	Blocked       ModuleState = "BLOCKED"
	Stale         ModuleState = "STALE"
)

type ModuleIO struct {
	Name       string `json:"name"`
	EntityType string `json:"entity_type"`
	Required   bool   `json:"required"`
}
type ModuleDependency struct {
	ModuleID ID     `json:"module_id"`
	Output   string `json:"output"`
	Required bool   `json:"required"`
	Stage    string `json:"stage"`
}
type ReviewGate struct {
	ID         string `json:"id"`
	Permission string `json:"permission"`
	Target     string `json:"target"`
}
type EngineeringModule struct {
	ID                   ID                 `json:"id"`
	Version              int                `json:"version"`
	Domain               Domain             `json:"domain"`
	Title                string             `json:"title"`
	Inputs               []ModuleIO         `json:"inputs"`
	Outputs              []ModuleIO         `json:"outputs"`
	Dependencies         []ModuleDependency `json:"dependencies"`
	Applicability        string             `json:"applicability"`
	Calculations         []string           `json:"calculations"`
	Validations          []string           `json:"validations"`
	ReviewGates          []ReviewGate       `json:"review_gates"`
	PublishTargets       []string           `json:"publish_targets"`
	ImplementationStatus string             `json:"implementation_status"`
}
type InputBinding struct {
	Name               string  `json:"name"`
	ProducerInstanceID *ID     `json:"producer_instance_id,omitempty"`
	Output             string  `json:"output"`
	RevisionID         *ID     `json:"revision_id,omitempty"`
	Digest             *string `json:"digest,omitempty"`
	Required           bool    `json:"required"`
	Available          bool    `json:"available"`
}
type ModuleInstance struct {
	ID                  ID             `json:"id"`
	DefinitionID        ID             `json:"definition_id"`
	DefinitionVersion   int            `json:"definition_version"`
	State               ModuleState    `json:"state"`
	ApplicabilityReason string         `json:"applicability_reason"`
	HeadRevisionID      *ID            `json:"head_revision_id,omitempty"`
	Inputs              []InputBinding `json:"inputs"`
}
type ImportPlan struct {
	ID             ID               `json:"id"`
	ProjectID      ID               `json:"project_id"`
	SourceID       ID               `json:"source_id"`
	Version        int              `json:"version"`
	Summary        string           `json:"summary"`
	PreviousPlanID *ID              `json:"previous_plan_id,omitempty"`
	Modules        []ModuleInstance `json:"modules"`
}

// Registry is constructed by trusted application code; AI may select definitions
// and propose bindings, but cannot register a calculator or publish target.
type Registry struct{ definitions map[string]EngineeringModule }

func NewRegistry(definitions []EngineeringModule) (*Registry, error) {
	r := &Registry{definitions: map[string]EngineeringModule{}}
	for _, d := range definitions {
		if d.ID == "" || d.Version < 1 || d.Applicability == "" || !validDomain(d.Domain) {
			return nil, fmt.Errorf("INVALID_MODULE_DEFINITION: %s", d.ID)
		}
		if _, ok := r.definitions[d.ID]; ok {
			return nil, fmt.Errorf("DUPLICATE_MODULE: %s", d.ID)
		}
		inputs, outputs := map[string]bool{}, map[string]bool{}
		for _, input := range d.Inputs {
			if input.Name == "" || input.EntityType == "" || inputs[input.Name] {
				return nil, fmt.Errorf("INVALID_MODULE_INPUT: %s", d.ID)
			}
			inputs[input.Name] = true
		}
		for _, output := range d.Outputs {
			if output.Name == "" || output.EntityType == "" || outputs[output.Name] {
				return nil, fmt.Errorf("INVALID_MODULE_OUTPUT: %s", d.ID)
			}
			outputs[output.Name] = true
		}
		r.definitions[d.ID] = cloneDefinition(d)
	}
	for _, d := range r.definitions {
		for _, dep := range d.Dependencies {
			producer, ok := r.definitions[dep.ModuleID]
			if !ok || !hasOutput(producer, dep.Output) || (dep.Stage != "process" && dep.Stage != "review" && dep.Stage != "publish") {
				return nil, fmt.Errorf("INVALID_MODULE_DEPENDENCY: %s", d.ID)
			}
		}
	}
	return r, nil
}
func cloneDefinition(d EngineeringModule) EngineeringModule {
	d.Inputs = append([]ModuleIO{}, d.Inputs...)
	d.Outputs = append([]ModuleIO{}, d.Outputs...)
	d.Dependencies = append([]ModuleDependency{}, d.Dependencies...)
	d.Calculations = append([]string{}, d.Calculations...)
	d.Validations = append([]string{}, d.Validations...)
	d.ReviewGates = append([]ReviewGate{}, d.ReviewGates...)
	d.PublishTargets = append([]string{}, d.PublishTargets...)
	return d
}
func (r *Registry) Definition(id string) (EngineeringModule, bool) {
	d, ok := r.definitions[id]
	return cloneDefinition(d), ok
}
func (r *Registry) Definitions() []EngineeringModule {
	out := []EngineeringModule{}
	for _, d := range r.definitions {
		out = append(out, cloneDefinition(d))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func hasOutput(d EngineeringModule, name string) bool {
	for _, o := range d.Outputs {
		if o.Name == name {
			return true
		}
	}
	return false
}
func validDomain(d Domain) bool {
	switch d {
	case SourceDomain, EquipmentDomain, FacilityDomain, PowerDomain, ConnectivityDomain, MaterialDomain, NetworkDomain, ReleaseDomain:
		return true
	}
	return false
}

// PlanBatches resolves the actual plan graph, not a hard-coded module order.
// An omitted required input blocks only that instance and its consumers. Results
// include independent ready branches even if another branch needs more material.
func (r *Registry) PlanBatches(plan ImportPlan) ([][]ID, map[ID][]string, error) {
	instances := map[ID]ModuleInstance{}
	for _, m := range plan.Modules {
		d, ok := r.definitions[m.DefinitionID]
		if !ok || d.Version != m.DefinitionVersion || m.ID == "" {
			return nil, nil, fmt.Errorf("UNKNOWN_MODULE_VERSION: %s", m.ID)
		}
		if _, ok := instances[m.ID]; ok {
			return nil, nil, fmt.Errorf("DUPLICATE_INSTANCE: %s", m.ID)
		}
		if !validState(m.State) || (m.State == NotApplicable && m.ApplicabilityReason == "") {
			return nil, nil, fmt.Errorf("INVALID_MODULE_STATE: %s", m.ID)
		}
		instances[m.ID] = m
	}
	edges := map[ID][]ID{}
	blocked := map[ID][]string{}
	for _, m := range plan.Modules {
		d := r.definitions[m.DefinitionID]
		seen := map[string]bool{}
		for _, input := range m.Inputs {
			declaration, found := ModuleIO{}, false
			for _, x := range d.Inputs {
				if x.Name == input.Name {
					declaration = x
					found = true
				}
			}
			if !found || seen[input.Name] || input.Required != declaration.Required {
				return nil, nil, fmt.Errorf("INVALID_INPUT_BINDING: %s/%s", m.ID, input.Name)
			}
			seen[input.Name] = true
			if input.ProducerInstanceID != nil {
				upstream, ok := instances[*input.ProducerInstanceID]
				if !ok {
					return nil, nil, fmt.Errorf("MISSING_PRODUCER: %s", m.ID)
				}
				pd := r.definitions[upstream.DefinitionID]
				matched := false
				for _, output := range pd.Outputs {
					if output.Name == input.Output && output.EntityType == declaration.EntityType {
						matched = true
					}
				}
				if !matched {
					return nil, nil, fmt.Errorf("INCOMPATIBLE_OUTPUT: %s/%s", m.ID, input.Name)
				}
				if input.Required {
					edges[m.ID] = append(edges[m.ID], upstream.ID)
				}
				if input.Required && (upstream.State == NotApplicable || upstream.State == Blocked || upstream.State == Stale) {
					blocked[m.ID] = append(blocked[m.ID], input.Name)
				}
			} else if input.Required && (!input.Available || input.RevisionID == nil || *input.RevisionID == "" || input.Digest == nil || *input.Digest == "") {
				blocked[m.ID] = append(blocked[m.ID], input.Name)
			}
		}
		for _, input := range d.Inputs {
			if input.Required && !seen[input.Name] {
				blocked[m.ID] = append(blocked[m.ID], input.Name)
			}
		}
		if m.State == Blocked || m.State == Stale || m.State == NotApplicable {
			blocked[m.ID] = append(blocked[m.ID], string(m.State))
		}
	}
	// Check the complete required graph even if a node is currently blocked.
	visited := map[ID]int{}
	var visit func(ID) error
	visit = func(id ID) error {
		if visited[id] == 1 {
			return fmt.Errorf("MODULE_DEPENDENCY_CYCLE: %s", id)
		}
		if visited[id] == 2 {
			return nil
		}
		visited[id] = 1
		for _, dep := range edges[id] {
			if e := visit(dep); e != nil {
				return e
			}
		}
		visited[id] = 2
		return nil
	}
	for id := range instances {
		if e := visit(id); e != nil {
			return nil, nil, e
		}
	}
	for changed := true; changed; {
		changed = false
		for id, deps := range edges {
			if len(blocked[id]) > 0 {
				continue
			}
			for _, dep := range deps {
				if len(blocked[dep]) > 0 {
					blocked[id] = append(blocked[id], "dependency:"+dep)
					changed = true
				}
			}
		}
	}
	batches := [][]ID{}
	done := map[ID]bool{}
	for {
		batch := []ID{}
		for id := range instances {
			if done[id] || len(blocked[id]) > 0 {
				continue
			}
			ready := true
			for _, dep := range edges[id] {
				if !done[dep] {
					ready = false
				}
			}
			if ready {
				batch = append(batch, id)
			}
		}
		if len(batch) == 0 {
			break
		}
		sort.Strings(batch)
		batches = append(batches, batch)
		for _, id := range batch {
			done[id] = true
		}
	}
	return batches, blocked, nil
}

// Invalidate returns only transitive consumers of changed instance outputs.
// Published history is retained; callers create a new head rather than mutating
// an approved revision. Optional consumed inputs also invalidate stale outputs.
func Invalidate(plan ImportPlan, changed []ID) []ID {
	affected := map[ID]bool{}
	for _, id := range changed {
		affected[id] = true
	}
	for more := true; more; {
		more = false
		for _, m := range plan.Modules {
			if affected[m.ID] {
				continue
			}
			for _, i := range m.Inputs {
				if i.ProducerInstanceID != nil && affected[*i.ProducerInstanceID] {
					affected[m.ID] = true
					more = true
					break
				}
			}
		}
	}
	out := []ID{}
	for id := range affected {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
func validState(s ModuleState) bool {
	switch s {
	case NotApplicable, Detected, NeedsInput, Ready, Processing, InReview, Approved, Published, Blocked, Stale:
		return true
	}
	return false
}

type TransitionEvidence struct {
	ActorType           string
	RevisionID          ID
	ReviewedRevisionID  ID
	Digest              string
	ReviewedDigest      string
	InputsCurrent       bool
	HasBlockers         bool
	ApplicabilityReason string
	ReceiptID           ID
}

// Transition does not perform authorization. HTTP/service callers must enforce
// RBAC and hold a revision lock; this enforces domain preconditions independently.
func Transition(from, to ModuleState, e TransitionEvidence) error {
	allowed := map[ModuleState][]ModuleState{
		Detected: {NeedsInput, Ready, NotApplicable, Blocked}, NeedsInput: {Ready, Blocked, NotApplicable},
		Ready: {Processing, NeedsInput, Blocked}, Processing: {InReview, NeedsInput, Blocked},
		InReview: {Approved, Blocked, Ready, Stale}, Approved: {Published, Stale}, Published: {Stale},
		Blocked: {Ready, NeedsInput, NotApplicable}, Stale: {Ready, NeedsInput, Blocked}, NotApplicable: {Detected},
	}
	ok := false
	for _, s := range allowed[from] {
		if s == to {
			ok = true
		}
	}
	if !ok {
		return fmt.Errorf("INVALID_MODULE_TRANSITION: %s -> %s", from, to)
	}
	if to == NotApplicable && e.ApplicabilityReason == "" {
		return fmt.Errorf("APPLICABILITY_EVIDENCE_REQUIRED")
	}
	if to == Ready && (!e.InputsCurrent || e.HasBlockers) {
		return fmt.Errorf("INPUTS_NOT_READY")
	}
	if to == Approved || to == Published {
		if e.HasBlockers || !e.InputsCurrent {
			return fmt.Errorf("RELEASE_BLOCKED")
		}
		if e.RevisionID == "" || e.Digest == "" || e.RevisionID != e.ReviewedRevisionID || e.Digest != e.ReviewedDigest {
			return fmt.Errorf("REVIEW_REVISION_MISMATCH")
		}
	}
	if to == Approved && e.ActorType != "human" {
		return fmt.Errorf("HUMAN_REVIEW_REQUIRED")
	}
	if to == Published && e.ReceiptID == "" {
		return fmt.Errorf("PUBLISH_RECEIPT_REQUIRED")
	}
	return nil
}

func DefaultRegistry() *Registry {
	defs := []EngineeringModule{
		{ID: "workbook_intelligence", Domain: SourceDomain, Title: "资料理解", Inputs: []ModuleIO{{"source", "WorkbookSource", true}}, Outputs: []ModuleIO{{"facts", "SemanticFact", false}, {"plan", "ImportPlan", false}}, Applicability: "存在开局资料", Validations: []string{"evidence", "coverage", "schema"}},
		{ID: "equipment", Domain: EquipmentDomain, Title: "设备与组合", Inputs: []ModuleIO{{"facts", "SemanticFact", true}}, Outputs: []ModuleIO{{"equipment", "EquipmentPlan", false}, {"assemblies", "DeviceAssembly", false}}, Applicability: "资料包含设备身份、型号、数量或部件组合", Validations: []string{"equipment_identity", "assembly_cycle", "source_coverage"}, PublishTargets: []string{"CMDB_PLANNED_EQUIPMENT"}},
		{ID: "facility_placement", Domain: FacilityDomain, Title: "机房与落位", Inputs: []ModuleIO{{"facts", "SemanticFact", true}, {"equipment", "EquipmentPlan", false}}, Outputs: []ModuleIO{{"facility", "FacilityModel", false}, {"placement", "RackPlacement", false}}, Applicability: "资料包含场地、空间坐标、机柜或安装位置", Validations: []string{"facility_hierarchy", "rack_capacity", "placement_overlap"}, PublishTargets: []string{"CMDB_PLANNED_FACILITY", "CMDB_PLANNED_PLACEMENT"}},
		{ID: "power_cooling", Domain: PowerDomain, Title: "电力与冷却", Inputs: []ModuleIO{{"facts", "SemanticFact", true}, {"equipment", "EquipmentPlan", false}, {"placement", "RackPlacement", false}}, Outputs: []ModuleIO{{"power", "PowerPlan", false}}, Applicability: "资料包含功率、负载、容量或冷却依据", Calculations: []string{"approved_power_rules"}, Validations: []string{"power_basis", "capacity", "units"}, PublishTargets: []string{"ENGINEERING_POWER"}},
		{ID: "connectivity_cabling", Domain: ConnectivityDomain, Title: "连接与布线", Inputs: []ModuleIO{{"facts", "SemanticFact", true}, {"equipment", "EquipmentPlan", true}, {"placement", "RackPlacement", false}}, Outputs: []ModuleIO{{"connectivity", "ConnectivityPlan", false}, {"cabling", "CablePlan", false}}, Applicability: "资料包含端接、端口、配线节点、路径或线缆需求", Calculations: []string{"approved_route_rules", "approved_stock_rules", "approved_optic_rules"}, Validations: []string{"endpoints", "port_occupancy", "maturity"}, PublishTargets: []string{"CMDB_PLANNED_CONNECTIVITY", "ENGINEERING_CABLING"}},
		{ID: "material_workload", Domain: MaterialDomain, Title: "采购与施工", Inputs: []ModuleIO{{"facts", "SemanticFact", true}, {"equipment", "EquipmentPlan", false}, {"cabling", "CablePlan", false}}, Outputs: []ModuleIO{{"material", "MaterialEstimate", false}, {"workload", "WorkloadEstimate", false}}, Applicability: "资料包含采购、需求、备件、安装或施工数量", Calculations: []string{"approved_procurement_rules", "approved_workload_rules"}, Validations: []string{"units", "estimate_not_asset"}, PublishTargets: []string{"ENGINEERING_MATERIAL", "ENGINEERING_WORKLOAD"}},
		{ID: "network_design", Domain: NetworkDomain, Title: "逻辑网络", Inputs: []ModuleIO{{"facts", "SemanticFact", true}, {"connectivity", "ConnectivityPlan", false}}, Outputs: []ModuleIO{{"network", "NetworkPlan", false}}, Applicability: "存在逻辑网络规划依据；物理链路本身不是协议依据", Validations: []string{"network_source", "allocations", "approved_physical_revision"}, PublishTargets: []string{"NETWORK_REVISION"}},
		{ID: "validation_release", Domain: ReleaseDomain, Title: "校验审核与发布", Inputs: []ModuleIO{{"plan", "ImportPlan", true}}, Outputs: []ModuleIO{{"issues", "ValidationIssue", false}, {"publication", "PublishRecord", false}}, Applicability: "存在工程模块及其产物", Validations: []string{"cross_domain", "review_version", "publish_target"}},
	}
	// Definitions declare producer/output dependencies. Actual instance bindings
	// may consume an existing version instead, so this is not a fixed stage order.
	for i := range defs {
		for _, input := range defs[i].Inputs {
			for _, producer := range defs {
				if producer.ID == defs[i].ID {
					continue
				}
				for _, output := range producer.Outputs {
					if output.EntityType == input.EntityType {
						defs[i].Dependencies = append(defs[i].Dependencies, ModuleDependency{producer.ID, output.Name, input.Required, "process"})
					}
				}
			}
		}
	}
	for i := range defs {
		defs[i].Version = 1
		defs[i].ImplementationStatus = "planned"
		for _, target := range defs[i].PublishTargets {
			defs[i].ReviewGates = append(defs[i].ReviewGates, ReviewGate{"human_revision_review", "bom:review", target})
		}
	}
	// These flags describe only the implemented kernel, not an end-to-end module.
	defs[1].ImplementationStatus = "model_and_validation"
	defs[2].ImplementationStatus = "model_and_validation"
	r, err := NewRegistry(defs)
	if err != nil {
		panic(err)
	}
	return r
}
