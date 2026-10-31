package flows

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Task struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Command string `json:"command"`
	SHA256  string `json:"sha256"`
	Timeout int    `json:"timeout_seconds"`
}
type Result struct {
	Digest string `json:"request_digest,omitempty"`
	ID     string `json:"id"`
	State  string `json:"state"`
	Exit   int    `json:"exit_code"`
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

func ResolveSecret(ref string) (string, error) {
	root := os.Getenv("KEEL_SSH_SECRET_DIR")
	if root == "" || filepath.Base(ref) != ref || ref == "." || ref == "" {
		return "", problem(422, "SECRET_REFERENCE_INVALID")
	}
	p := filepath.Join(root, ref)
	info, e := os.Lstat(p)
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return "", problem(422, "SECRET_FILE_NOT_CONTROLLED")
	}
	return p, nil
}

type bounded struct {
	mu    sync.Mutex
	b     bytes.Buffer
	limit int
}

func (b *bounded) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	if b.b.Len() < b.limit {
		end := n
		if end > b.limit-b.b.Len() {
			end = b.limit - b.b.Len()
		}
		b.b.Write(p[:end])
	}
	return n, nil
}
func (b *bounded) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.b.String() }
func RunSSH(ctx context.Context, host string, port int, user, keyRef, knownRef string, task Task) (Result, error) {
	out := Result{ID: task.ID, State: "FAILED", Exit: -1}
	keyPath, e := ResolveSecret(keyRef)
	if e != nil {
		return out, e
	}
	knownPath, e := ResolveSecret(knownRef)
	if e != nil {
		return out, e
	}
	key, e := os.ReadFile(keyPath)
	if e != nil {
		return out, e
	}
	signer, e := ssh.ParsePrivateKey(key)
	if e != nil {
		return out, problem(422, "SSH_KEY_INVALID")
	}
	verify, e := knownhosts.New(knownPath)
	if e != nil {
		return out, e
	}
	config := &ssh.ClientConfig{User: user, Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, HostKeyCallback: verify, Timeout: 10 * time.Second}
	address := net.JoinHostPort(host, fmt.Sprint(port))
	conn, e := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", address)
	if e != nil {
		return out, problem(502, "SSH_CONNECT_FAILED")
	}
	conn.SetDeadline(time.Now().Add(time.Duration(task.Timeout) * time.Second))
	c, ch, req, e := ssh.NewClientConn(conn, address, config)
	if e != nil {
		conn.Close()
		return out, problem(502, "SSH_TRUST_OR_AUTH_FAILED")
	}
	client := ssh.NewClient(c, ch, req)
	defer client.Close()
	session, e := client.NewSession()
	if e != nil {
		return out, e
	}
	defer session.Close()
	stdout, stderr := &bounded{limit: 1 << 20}, &bounded{limit: 1 << 20}
	session.Stdout = stdout
	session.Stderr = stderr
	script := "sh -s"
	if task.Kind == "bash" {
		script = "bash -s"
	}
	if task.Kind == "python" {
		script = "python3 -"
	}
	if task.Kind == "command" {
		script = task.Command
	} else {
		session.Stdin = strings.NewReader(task.Command)
	}
	done := make(chan error, 1)
	go func() { done <- session.Run(script) }()
	select {
	case <-ctx.Done():
		session.Signal(ssh.SIGTERM)
		client.Close()
		out.State = "RECONCILING"
		out.Stderr = "Channel closed; remote side effects require reconciliation"
		return out, nil
	case e = <-done:
	}
	out.Stdout = stdout.String()
	out.Stderr = stderr.String()
	out.Exit = 0
	if e != nil {
		out.Exit = -1
		if ex, ok := e.(*ssh.ExitError); ok {
			out.Exit = ex.ExitStatus()
		} else {
			out.State = "RECONCILING"
			return out, nil
		}
	}
	if out.Exit == 0 {
		out.State = "SUCCESS"
	}
	return out, nil
}
func (s *Service) Dispatch(ctx context.Context, token, asset string, required ...string) (Result, error) {
	var id string
	e := s.DB.QueryRow(ctx, `SELECT execution_id::text FROM flow_dispatch_ticket WHERE hash=$1 AND expires_at>now()`, hash(token)).Scan(&id)
	if e != nil {
		return Result{}, problem(403, "DISPATCH_TICKET_INVALID")
	}
	var b, snapshot []byte
	var transport, state string
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback(ctx)
	e = tx.QueryRow(ctx, `SELECT t.snapshot,t.selected_transport,t.state,b.request FROM flow_execution_target t JOIN flow_execution_binding b ON b.id=t.execution_id WHERE t.execution_id=$1 AND t.asset_id=$2 FOR UPDATE OF t`, id, asset).Scan(&snapshot, &transport, &state, &b)
	if e != nil {
		return Result{}, problem(404, "TARGET_NOT_FOUND")
	}
	if len(required) > 0 && required[0] != "" && required[0] != transport {
		return Result{}, problem(409, "FROZEN_TRANSPORT_MISMATCH")
	}
	if state != "PENDING" {
		return Result{}, problem(409, "TARGET_ALREADY_DISPATCHED_RECONCILE")
	}
	var r RunRequest
	json.Unmarshal(b, &r)
	var t Target
	json.Unmarshal(snapshot, &t)
	// Recheck live lifecycle/identity before dispatch, not only the frozen selection.
	var payload []byte
	var revision int64
	e = tx.QueryRow(ctx, `SELECT payload || jsonb_build_object('maintenance_state',CASE WHEN maintenance_state='maintained' OR payload->>'maintenance_state'='maintained' OR payload->'fields'->>'maintenance_state'='maintained' THEN 'maintained' ELSE maintenance_state END,'lifecycle_state',CASE WHEN lifecycle_state='maintained' OR payload->>'lifecycle_state'='maintained' OR payload->'fields'->>'lifecycle_state'='maintained' THEN 'maintained' ELSE lifecycle_state END),version FROM cmdb_entity WHERE item_uid=$1 AND EXISTS(SELECT 1 FROM cmdb_asset_scope WHERE asset_id=item_uid AND project_id=$2)`, asset, r.Project).Scan(&payload, &revision)
	if e != nil {
		return Result{}, problem(404, "TARGET_NOT_FOUND")
	}
	var live Target
	json.Unmarshal(payload, &live.Fields)
	if Maintained(live) || revision != t.Revision {
		return Result{}, problem(409, "TARGET_FACT_CHANGED")
	}
	_, e = tx.Exec(ctx, `UPDATE flow_execution_target SET state='RUNNING',attempt=attempt+1,actual_transport=selected_transport,started_at=now() WHERE execution_id=$1 AND asset_id=$2`, id, asset)
	if e != nil {
		return Result{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return Result{}, e
	}
	task := Task{ID: id + ":" + asset, Kind: r.Kind, Command: r.Command, SHA256: hash(r.Command), Timeout: r.Timeout}
	taskCtx, cancel := context.WithTimeout(ctx, time.Duration(r.Timeout)*time.Second)
	defer cancel()
	var result Result
	if transport == "AGENT" {
		agentTask := uuid.NewString()
		_, e = s.DB.Exec(ctx, `INSERT INTO flow_agent_task(id,agent_id,execution_id,asset_id,state,command_hash) VALUES($1,$2,$3,$4,'DISPATCHED',$5)`, agentTask, t.Agent, id, asset, hash(r.Command))
		if e != nil {
			return Result{}, e
		}
		s.DB.Exec(ctx, `UPDATE flow_execution_target SET agent_task_id=$3 WHERE execution_id=$1 AND asset_id=$2`, id, asset, agentTask)
		result, e = s.Gateway.Execute(taskCtx, t.Agent, task)
	} else {
		var host, user, key, known string
		var port int
		e = s.DB.QueryRow(ctx, `SELECT host,port,username,secret_ref,known_hosts_ref FROM flow_ssh_trust WHERE id::text=$1 AND project_id=$2 AND asset_id=$3 AND status='approved'`, t.SSH, t.Project, t.ID).Scan(&host, &port, &user, &key, &known)
		if e == nil {
			result, e = RunSSH(taskCtx, host, port, user, key, known, task)
		}
	}
	if transport == "AGENT" {
		s.DB.Exec(context.WithoutCancel(ctx), `UPDATE flow_agent_task SET state=$3,result=$4,updated_at=now() WHERE execution_id=$1 AND asset_id=$2`, id, asset, result.State, raw(result))
	}
	if e != nil {
		result = Result{ID: task.ID, State: "FAILED", Exit: -1, Stderr: "Transport unavailable; see controlled audit"}
	}
	if result.State == "" {
		result.State = "RECONCILING"
	}
	persistCtx := context.WithoutCancel(ctx)
	resultTx, persistErr := s.DB.Begin(persistCtx)
	if persistErr != nil {
		return result, persistErr
	}
	defer resultTx.Rollback(persistCtx)
	_, persistErr = resultTx.Exec(persistCtx, `UPDATE flow_execution_target SET state=$3,exit_code=$4,stdout=$5,stderr=$6,ended_at=now() WHERE execution_id=$1 AND asset_id=$2`, id, asset, result.State, result.Exit, redactOutput(result.Stdout), redactOutput(result.Stderr))
	if persistErr != nil {
		return result, persistErr
	}
	_, persistErr = resultTx.Exec(persistCtx, `INSERT INTO outbox_event(id,project_id,event_type,payload) VALUES(gen_random_uuid(),$1,'flows.execution',$2)`, r.Project, raw(ProjectionEvent{Execution: id, Targets: []string{asset}, Actor: "job-service", State: result.State}))
	if persistErr != nil {
		return result, persistErr
	}
	if persistErr = resultTx.Commit(persistCtx); persistErr != nil {
		return result, persistErr
	}
	return result, nil
}
func redactOutput(v string) string {
	for _, name := range []string{"KESTRA_PASSWORD", "DATABASE_URL", "KEEL_BOOTSTRAP_PASSWORD"} {
		if secret := os.Getenv(name); secret != "" {
			v = strings.ReplaceAll(v, secret, "[REDACTED]")
		}
	}
	return v
}
func Digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

var _ io.Writer = (*bounded)(nil)
