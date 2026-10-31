package flows

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type BootstrapRequest struct {
	Project string `json:"project_id"`
	Asset   string `json:"asset_id"`
	Version string `json:"version"`
}

func (s *Service) Bootstrap(ctx context.Context, p Scope, in BootstrapRequest) (map[string]any, error) {
	if e := requireProject(p, in.Project); e != nil {
		return nil, e
	}
	if in.Version != "0.1.0" {
		return nil, problem(422, "AGENT_VERSION_UNAVAILABLE")
	}
	url := os.Getenv("KEEL_AGENT_PACKAGE_URL")
	sha := os.Getenv("KEEL_AGENT_PACKAGE_SHA256")
	api := os.Getenv("KEEL_PUBLIC_HTTPS_URL")
	gateway := os.Getenv("KEEL_AGENT_PUBLIC_GATEWAY")
	if !strings.HasPrefix(url, "https://") || len(sha) != 64 || !strings.HasPrefix(api, "https://") || gateway == "" {
		return nil, problem(503, "BOOTSTRAP_PACKAGE_OR_TLS_NOT_CONFIGURED")
	}
	for _, v := range []string{url, sha, api, gateway} {
		if strings.ContainsAny(v, "'\n\r") {
			return nil, problem(422, "BOOTSTRAP_CONFIGURATION_INVALID")
		}
	}
	targets, e := s.Targets(ctx, Scope{Projects: []string{in.Project}}, "")
	if e != nil {
		return nil, e
	}
	var target *Target
	for _, t := range targets {
		if t.ID == in.Asset {
			copy := t
			target = &copy
		}
	}
	if target == nil || target.Type == "switch" || Maintained(*target) || target.SSH == "" {
		return nil, problem(422, "BOOTSTRAP_TARGET_NOT_READY")
	}
	var active bool
	if e = s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM flow_agent_task WHERE asset_id=$1 AND state IN('DISPATCHED','RUNNING','RECONCILING'))`, in.Asset).Scan(&active); e != nil {
		return nil, e
	}
	if active {
		return nil, problem(409, "AGENT_ACTIVE_TASKS_BLOCK_BOOTSTRAP")
	}
	var host, user, key, known string
	var port int
	e = s.DB.QueryRow(ctx, `SELECT host,port,username,secret_ref,known_hosts_ref FROM flow_ssh_trust WHERE id::text=$1 AND status='approved'`, target.SSH).Scan(&host, &port, &user, &key, &known)
	if e != nil {
		return nil, e
	}
	issued, e := s.Token(ctx, p, in.Project, in.Asset)
	if e != nil {
		return nil, e
	}
	token := issued["token"].(string)
	// A preinstalled root-owned helper is the sole permitted sudo command. Script never prints the token.
	script := fmt.Sprintf("set -eu\numask 077\nd=$(mktemp -d)\ntrap 'rm -f \"$d/agent\" \"$d/token\"; rmdir \"$d\"' EXIT\ncurl --fail --silent --show-error --location '%s' -o \"$d/agent\"\nprintf '%%s  %%s\\n' '%s' \"$d/agent\" | sha256sum -c - >/dev/null\nprintf '%%s' '%s' > \"$d/token\"\nsudo -n /usr/local/sbin/keel-agent-bootstrap \"$d/agent\" \"$d/token\" '%s' '%s'\n", url, sha, token, api, gateway)
	task := Task{ID: "bootstrap:" + target.ID, Kind: "bash", Command: script, SHA256: hash(script), Timeout: 180}
	result, e := RunSSH(ctx, host, port, user, key, known, task)
	s.audit(ctx, p, in.Project, "agent.bootstrap", in.Asset, map[string]any{"version": in.Version, "state": result.State, "exit_code": result.Exit})
	if e != nil {
		return nil, e
	}
	if result.State != "SUCCESS" {
		return nil, problem(422, "BOOTSTRAP_FAILED_REVIEW_REMOTE_STATE")
	}
	return map[string]any{"asset_id": in.Asset, "state": "awaiting_registration", "version": in.Version}, nil
}
