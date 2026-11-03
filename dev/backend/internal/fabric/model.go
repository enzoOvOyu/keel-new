package fabric

import "time"

type Blueprint struct {
	ID         string    `json:"id"`
	FabricID   string    `json:"fabricId"`
	Revision   int       `json:"revision"`
	Input      string    `json:"inputSnapshotId"`
	Topology   string    `json:"topologyType"`
	PlaneCount int       `json:"planeCount"`
	TierCount  int       `json:"tierCount"`
	Groups     []string  `json:"deviceGroupIds"`
	Profiles   []string  `json:"profileRevisionIds"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"createdAt"`
}
type Profile struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Domain      string   `json:"domain"`
	Roles       []string `json:"roleSelector"`
	Policies    []string `json:"policyRefs"`
	Constraints []string `json:"capabilityConstraints"`
	Schema      string   `json:"schemaVersion"`
	Status      string   `json:"status"`
	Features    []string `json:"features"`
	Description string   `json:"description"`
}
type Parameters struct {
	ASN     *int64   `json:"asn"`
	Address string   `json:"address"`
	PFC     *int     `json:"pfc"`
	ECN     string   `json:"ecn"`
	Peers   []Peer   `json:"bgpPeers"`
	VRF     string   `json:"vrf"`
	VNI     *int     `json:"vni"`
	Sources []string `json:"sourceRefs"`
}
type Peer struct {
	Device  string `json:"deviceId"`
	ASN     int64  `json:"asn"`
	Address string `json:"address"`
}
type SemanticChange struct {
	Field     string `json:"field"`
	Before    string `json:"before"`
	After     string `json:"after"`
	Operation string `json:"operation"`
	Source    string `json:"sourceRef"`
}
type Artifact struct {
	ID             string           `json:"id"`
	Device         string           `json:"deviceId"`
	Domain         string           `json:"domain"`
	Format         string           `json:"format"`
	Hash           string           `json:"hash"`
	Sources        []string         `json:"sourceRefs"`
	Capability     string           `json:"capabilityRevision"`
	ContentRef     string           `json:"contentRef"`
	Validation     string           `json:"validationStatus"`
	Content        string           `json:"content"`
	Commands       string           `json:"commands"`
	Changes        []SemanticChange `json:"changes"`
	Unsupported    []string         `json:"unsupported"`
	Missing        []string         `json:"missing"`
	Profile        string           `json:"profileId"`
	ProfileVersion string           `json:"profileVersion"`
	Input          string           `json:"inputSnapshotId"`
	Parameters     Parameters       `json:"parameters"`
}
type Build struct {
	ID        string    `json:"id"`
	Blueprint string    `json:"blueprintRevisionId"`
	Input     string    `json:"inputSnapshotId"`
	Profiles  []string  `json:"profileRevisionIds"`
	Compiler  string    `json:"compilerVersion"`
	Status    string    `json:"status"`
	Artifacts []string  `json:"artifactIds"`
	CreatedAt time.Time `json:"createdAt"`
	Targets   []string  `json:"targetDeviceIds"`
}
type Finding struct {
	ID       string `json:"id"`
	Device   string `json:"deviceId,omitempty"`
	Check    string `json:"checkId"`
	Domain   string `json:"domain"`
	Status   string `json:"status"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Source   string `json:"sourceRef,omitempty"`
	Message  string `json:"message"`
	Evidence string `json:"evidenceRef,omitempty"`
	Required bool   `json:"required"`
}
type Validation struct {
	ID          string    `json:"id"`
	Build       string    `json:"configBuildId"`
	Status      string    `json:"status"`
	Passed      int       `json:"passed"`
	Warnings    int       `json:"warnings"`
	Blocked     int       `json:"blocked"`
	Unsupported int       `json:"unsupported"`
	Findings    []Finding `json:"findings"`
	Evidence    []string  `json:"evidenceRefs"`
	CreatedAt   time.Time `json:"createdAt"`
}
type Wave struct {
	ID          string   `json:"id"`
	Sequence    int      `json:"sequence"`
	Targets     []string `json:"targetDeviceIds"`
	Tasks       []string `json:"taskBindings"`
	Concurrency int      `json:"maxConcurrency"`
	Executions  []string `json:"executionRefs"`
	Depends     []string `json:"dependsOn"`
}
type Rollout struct {
	ID          string   `json:"id"`
	Strategy    string   `json:"strategy"`
	Waves       []Wave   `json:"waves"`
	Concurrency int      `json:"maxConcurrency"`
	Timeout     int      `json:"timeoutSeconds"`
	Retry       string   `json:"retryPolicyRef"`
	Safe        bool     `json:"safe"`
	Warnings    []string `json:"warnings"`
}
type Approval struct {
	ActorID   string    `json:"actorId"`
	ActorName string    `json:"actorName"`
	Decision  string    `json:"decision"`
	Reason    string    `json:"reason"`
	At        time.Time `json:"at"`
}
type Change struct {
	ID              string    `json:"id"`
	Build           string    `json:"configBuildId"`
	Validation      string    `json:"validationRunId"`
	Targets         []string  `json:"targetDeviceIds"`
	Risk            string    `json:"riskLevel"`
	Status          string    `json:"approvalStatus"`
	Rollout         string    `json:"rolloutPlanId"`
	Input           string    `json:"sourceSnapshotId"`
	Purpose         string    `json:"purpose"`
	CreatedBy       string    `json:"createdBy"`
	CreatedAt       time.Time `json:"createdAt"`
	Window          string    `json:"window"`
	Approval        *Approval `json:"approval,omitempty"`
	Task            string    `json:"taskBindingId"`
	Acknowledgement string    `json:"warningAcknowledgement"`
	Execution       string    `json:"executionId,omitempty"`
	ApprovalID      string    `json:"approvalId,omitempty"`
}
type Stage struct {
	Label string     `json:"label"`
	State string     `json:"state"`
	At    *time.Time `json:"at,omitempty"`
}
type Enrollment struct {
	ID             string            `json:"id"`
	Device         string            `json:"deviceId"`
	Profile        string            `json:"profileId"`
	ProfileVersion string            `json:"profileRevision"`
	Artifact       string            `json:"bootstrapArtifactId"`
	Address        string            `json:"managementAddress,omitempty"`
	Identity       map[string]string `json:"discoveredIdentity"`
	Matched        string            `json:"matchedDeviceId,omitempty"`
	Execution      string            `json:"jobExecutionId,omitempty"`
	Status         string            `json:"status"`
	Evidence       []string          `json:"evidenceRefs"`
	Timeline       []Stage           `json:"timeline"`
	Attempt        int               `json:"attempt"`
	Error          string            `json:"error,omitempty"`
}
type TargetRun struct {
	Device  string     `json:"deviceId"`
	State   string     `json:"state"`
	Attempt int        `json:"attempt"`
	Stage   string     `json:"stage"`
	Error   string     `json:"error,omitempty"`
	Logs    []string   `json:"logs"`
	Outputs []string   `json:"outputRefs"`
	Started *time.Time `json:"startedAt,omitempty"`
	Ended   *time.Time `json:"completedAt,omitempty"`
}
type Execution struct {
	Provider    string      `json:"provider"`
	ID          string      `json:"executionId"`
	Flow        string      `json:"flowId"`
	Revision    int         `json:"flowRevision"`
	TaskRuns    []string    `json:"taskRunIds"`
	State       string      `json:"state"`
	RawState    string      `json:"rawState"`
	LogRef      string      `json:"logRef"`
	Outputs     []string    `json:"outputRefs"`
	Started     time.Time   `json:"startedAt"`
	Ended       *time.Time  `json:"completedAt,omitempty"`
	Targets     []TargetRun `json:"targets"`
	Change      string      `json:"changeSetId"`
	Wave        int         `json:"wave"`
	FailureMode string      `json:"failureMode"`
}
type Comparison struct {
	Device      string     `json:"deviceId"`
	Artifact    string     `json:"artifactId"`
	Desired     string     `json:"desired"`
	Rendered    string     `json:"rendered"`
	Applied     string     `json:"applied"`
	Operational string     `json:"operational"`
	State       string     `json:"state"`
	Findings    []Finding  `json:"findings"`
	ObservedAt  *time.Time `json:"observedAt"`
	Source      string     `json:"source"`
}
type Assurance struct {
	ID         string       `json:"id"`
	Build      string       `json:"buildId"`
	Input      string       `json:"inputSnapshotId"`
	Executions []string     `json:"executionIds"`
	CreatedAt  time.Time    `json:"createdAt"`
	Records    []Comparison `json:"records"`
	Status     string       `json:"status"`
}
type Drift struct {
	ID       string    `json:"id"`
	Device   string    `json:"deviceId"`
	Field    string    `json:"field"`
	Expected string    `json:"expected"`
	Observed string    `json:"observed"`
	At       time.Time `json:"at"`
	State    string    `json:"state"`
	Source   string    `json:"sourceRef"`
}
type Ready struct {
	ID          string    `json:"id"`
	Input       string    `json:"inputSnapshotId"`
	Build       string    `json:"buildId"`
	Assurance   string    `json:"assuranceId"`
	Targets     []string  `json:"deviceIds"`
	CreatedAt   time.Time `json:"createdAt"`
	Status      string    `json:"status"`
	Limitations []string  `json:"limitations"`
}
type Audit struct {
	ID     string    `json:"id"`
	At     time.Time `json:"at"`
	Actor  string    `json:"actor"`
	Action string    `json:"action"`
	Ref    string    `json:"ref"`
}
type State struct {
	Schema      int          `json:"schemaVersion"`
	Project     string       `json:"projectId"`
	Actor       string       `json:"actorId"`
	Input       *string      `json:"inputId"`
	Blueprints  []Blueprint  `json:"blueprints"`
	Builds      []Build      `json:"builds"`
	Artifacts   []Artifact   `json:"artifacts"`
	Validations []Validation `json:"validations"`
	Changes     []Change     `json:"changes"`
	Rollouts    []Rollout    `json:"rollouts"`
	Enrollments []Enrollment `json:"enrollments"`
	Executions  []Execution  `json:"executions"`
	Assurances  []Assurance  `json:"assurances"`
	Drifts      []Drift      `json:"drifts"`
	Ready       []Ready      `json:"readyRecords"`
	Audit       []Audit      `json:"audit"`
	Connected   bool         `json:"providerConnected"`
	Profiles    []Profile    `json:"profileRevisions"`
}

func NewState(project string) State {
	return State{Schema: 1, Project: project, Blueprints: []Blueprint{}, Builds: []Build{}, Artifacts: []Artifact{}, Validations: []Validation{}, Changes: []Change{}, Rollouts: []Rollout{}, Enrollments: []Enrollment{}, Executions: []Execution{}, Assurances: []Assurance{}, Drifts: []Drift{}, Ready: []Ready{}, Audit: []Audit{}, Profiles: []Profile{}}
}

type Command struct {
	Action   string   `json:"action"`
	Revision int64    `json:"expectedRevision"`
	ID       string   `json:"id,omitempty"`
	IDs      []string `json:"ids,omitempty"`
	Domain   string   `json:"domain,omitempty"`
	Profile  string   `json:"profile,omitempty"`
	Task     string   `json:"task,omitempty"`
	Strategy string   `json:"strategy,omitempty"`
	Purpose  string   `json:"purpose,omitempty"`
	Window   string   `json:"window,omitempty"`
	Ack      string   `json:"acknowledgement,omitempty"`
	Reason   string   `json:"reason,omitempty"`
	Decision string   `json:"decision,omitempty"`
	Control  string   `json:"control,omitempty"`
}
