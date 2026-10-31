package flows

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Error struct {
	Status int
	Code   string
}

func (e *Error) Error() string              { return e.Code }
func problem(status int, code string) error { return &Error{status, code} }

type Scope struct {
	User, Actor string
	Projects    []string
}
type Service struct {
	DB      *pgxpool.Pool
	Engine  *Kestra
	Gateway *Gateway
}
type Flow struct {
	ID          string         `json:"id"`
	Project     string         `json:"project_id"`
	Namespace   string         `json:"namespace"`
	FlowID      string         `json:"flow_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	TargetType  string         `json:"target_type"`
	Revision    int            `json:"revision"`
	Source      string         `json:"source"`
	Disabled    bool           `json:"disabled"`
	Engine      map[string]any `json:"engine"`
}
type SaveFlow struct {
	Project     string `json:"project_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TargetType  string `json:"target_type"`
	Source      string `json:"source"`
	Revision    int    `json:"expected_revision"`
}
type Target struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Type         string         `json:"type"`
	Project      string         `json:"project_id"`
	Revision     int64          `json:"revision"`
	Fields       map[string]any `json:"fields"`
	Agent        string         `json:"agent_id"`
	AgentOnline  bool           `json:"agent_online"`
	Capabilities []string       `json:"capabilities"`
	SSH          string         `json:"ssh_profile_id"`
}
type RunRequest struct {
	Project     string   `json:"project_id"`
	Flow        string   `json:"flow_id"`
	Revision    int      `json:"expected_revision"`
	Targets     []string `json:"target_ids"`
	Kind        string   `json:"kind"`
	Command     string   `json:"command"`
	Policy      string   `json:"transport_policy"`
	Timeout     int      `json:"timeout_seconds"`
	Concurrency int      `json:"target_concurrency"`
	Retries     int      `json:"max_retries"`
	Source      string   `json:"source_module"`
	Parent      string   `json:"parent_id,omitempty"`
}
type Decision struct {
	Asset     string `json:"asset_id"`
	Name      string `json:"name"`
	Revision  int64  `json:"revision"`
	Transport string `json:"transport"`
	Reason    string `json:"reason"`
	Ready     bool   `json:"ready"`
	Target    Target `json:"target"`
}
type TargetRun struct {
	ID       string     `json:"id"`
	Snapshot Target     `json:"snapshot"`
	Selected string     `json:"selected_transport"`
	Actual   *string    `json:"actual_transport"`
	State    string     `json:"state"`
	Attempt  int        `json:"attempt"`
	Exit     *int       `json:"exit_code"`
	Stdout   string     `json:"stdout"`
	Stderr   string     `json:"stderr"`
	Started  *time.Time `json:"started_at"`
	Ended    *time.Time `json:"ended_at"`
}
type Execution struct {
	ID        string         `json:"id"`
	Project   string         `json:"project_id"`
	FlowID    *string        `json:"flow_id"`
	KestraID  *string        `json:"kestra_id"`
	Flow      string         `json:"engine_flow_id"`
	Revision  int            `json:"flow_revision"`
	Source    string         `json:"source_module"`
	Mode      string         `json:"mode"`
	Requester string         `json:"started_by"`
	Created   time.Time      `json:"created_at"`
	Error     *string        `json:"error_code"`
	Engine    map[string]any `json:"engine"`
	Targets   []TargetRun    `json:"targets"`
}

func hash(v string) string { s := sha256.Sum256([]byte(v)); return hex.EncodeToString(s[:]) }
func raw(v any) []byte     { b, _ := json.Marshal(v); return b }
func (s *Service) audit(ctx context.Context, p Scope, project, action, id string, details any) error {
	_, e := s.DB.Exec(ctx, `INSERT INTO flow_audit(id,project_id,actor,action,resource_id,details) VALUES(gen_random_uuid(),$1,$2,$3,$4,$5)`, project, p.Actor, action, id, raw(details))
	return e
}
func requireProject(scope Scope, p string) error {
	for _, v := range scope.Projects {
		if v == p {
			return nil
		}
	}
	return problem(403, "FLOWS_SCOPE_DENIED")
}
func field(t Target, k string) string {
	v := t.Fields[k]
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func Maintained(t Target) bool {
	for _, k := range []string{"lifecycle_status", "lifecycle_state", "maintenance_state", "lifecycle", "status"} {
		if field(t, k) == "maintained" {
			return true
		}
	}
	return false
}
func Route(t Target, policy, capability string) Decision {
	d := Decision{Asset: t.ID, Name: t.Name, Revision: t.Revision, Target: t, Transport: "BLOCKED"}
	if Maintained(t) {
		d.Reason = "MAINTAINED_IPMI_ONLY"
		return d
	}
	agent := t.Type != "switch" && t.AgentOnline
	has := false
	for _, c := range t.Capabilities {
		if c == capability {
			has = true
		}
	}
	if policy != "ssh_only" && agent && has {
		d.Transport = "AGENT"
		d.Ready = true
		return d
	}
	if policy != "agent_only" && t.SSH != "" {
		d.Transport = "SSH"
		d.Ready = true
		return d
	}
	d.Reason = "NO_APPROVED_TRANSPORT"
	return d
}
func runValid(r RunRequest) error {
	if len(r.Targets) == 0 || len(r.Targets) > 200 {
		return problem(422, "TARGET_COUNT_INVALID")
	}
	if r.Command == "" || len(r.Command) > 131072 {
		return problem(422, "COMMAND_INVALID")
	}
	if r.Kind != "command" && r.Kind != "bash" && r.Kind != "python" {
		return problem(422, "SCRIPT_KIND_INVALID")
	}
	if r.Policy != "agent_preferred" && r.Policy != "agent_only" && r.Policy != "ssh_only" {
		return problem(422, "TRANSPORT_POLICY_INVALID")
	}
	if r.Timeout < 1 || r.Timeout > 3600 || r.Concurrency < 1 || r.Concurrency > 64 || r.Retries < 0 || r.Retries > 3 {
		return problem(422, "EXECUTION_LIMIT_INVALID")
	}
	return nil
}

var _ = errors.Is
