package flows

import (
	"context"
	"go.yaml.in/yaml/v3"
	"testing"
)

func TestTransportPolicy(t *testing.T) {
	base := Target{ID: "synthetic-server", Type: "server", AgentOnline: true, SSH: "trusted", Capabilities: []string{"script.bash"}}
	for _, test := range []struct {
		name, policy string
		target       Target
		ready        bool
		transport    string
	}{{"preferred online", "agent_preferred", base, true, "AGENT"}, {"ssh forced", "ssh_only", base, true, "SSH"}, {"maintained blocks both", "agent_preferred", func() Target { x := base; x.Fields = map[string]any{"lifecycle_state": "maintained"}; return x }(), false, "BLOCKED"}, {"switch never agent", "agent_preferred", func() Target { x := base; x.Type = "switch"; return x }(), true, "SSH"}, {"capability missing", "agent_only", func() Target { x := base; x.Capabilities = nil; return x }(), false, "BLOCKED"}, {"offline no blind fallback", "agent_only", func() Target { x := base; x.AgentOnline = false; return x }(), false, "BLOCKED"}} {
		t.Run(test.name, func(t *testing.T) {
			d := Route(test.target, test.policy, "script.bash")
			if d.Ready != test.ready || d.Transport != test.transport {
				t.Fatalf("unexpected decision %#v", d)
			}
		})
	}
}
func TestBridgeDefinition(t *testing.T) {
	source := bridgeSource("keel.synthetic", "target-job", "http://internal", RunRequest{Concurrency: 8, Timeout: 60})
	var d map[string]any
	if e := yaml.Unmarshal([]byte(source), &d); e != nil {
		t.Fatal(e)
	}
	var task map[string]any
	for _, value := range d["tasks"].([]any) {
		candidate := value.(map[string]any)
		if candidate["id"] == "targets" {
			task = candidate
		}
	}
	if task == nil {
		t.Fatal("missing target loop")
	}
	branch := task["tasks"].([]any)[0].(map[string]any)
	if branch["type"] != "io.kestra.plugin.core.flow.If" {
		t.Fatal("missing frozen transport branch")
	}
	if task["concurrencyLimit"] != 8 || task["type"] != "io.kestra.plugin.core.flow.Loop" {
		t.Fatal(task)
	}
	if _, e := FlowDocument(source); e != nil {
		t.Fatal(e)
	}
}

func TestProjectNamespace(t *testing.T) {
	if Namespace("proj-a") == Namespace("proja") {
		t.Fatal("project namespace collision")
	}
}

func TestProjectPolicy(t *testing.T) {
	for _, source := range []string{"id: scoped\nnamespace: keel.test\ntasks:\n- id: leak\n  type: io.kestra.plugin.core.log.Log\n  message: '{{ envs.DATABASE_URL }}'\n", "id: scoped\nnamespace: keel.test\ntasks:\n- id: cross\n  type: io.kestra.plugin.core.flow.Subflow\n  namespace: another.project\n  flowId: secret\n"} {
		if ValidatePolicy(source, "keel.test") == nil {
			t.Fatal("unsafe engine/project scope accepted")
		}
	}
}

func TestAcceptedAgentDisconnectIsUnknown(t *testing.T) {
	connection := &agentConnection{send: make(chan AgentMessage, 1), pending: map[string]chan Result{}, done: make(chan struct{})}
	gateway := &Gateway{connections: map[string]*agentConnection{"synthetic": connection}}
	go func() { <-connection.send; close(connection.done) }()
	result, e := gateway.Execute(context.Background(), "synthetic", Task{ID: "accepted-task"})
	if e != nil || result.State != "RECONCILING" {
		t.Fatalf("accepted disconnect must not become a retryable failure: %#v %v", result, e)
	}
}

func TestSwitchChangesCannotBypassFabricApproval(t *testing.T) {
	if !SwitchReadOnly("nv show interface swp1 qos roce status\nshow interfaces") {
		t.Fatal("read-only diagnostics rejected")
	}
	for _, command := range []string{"nv config apply", "nv show; reboot", "show $(id)", "nv show\nconfigure terminal"} {
		if SwitchReadOnly(command) {
			t.Fatal("unsafe switch command accepted", command)
		}
	}
}
