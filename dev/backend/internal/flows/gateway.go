package flows

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/structpb"
	"net"
	"strings"
	"sync"
)

const GatewayMethod = "/keel.agent.v1.Gateway/Connect"

type AgentLog struct {
	TaskID string `json:"task_id"`
	Stream string `json:"stream"`
	Chunk  string `json:"chunk"`
}
type AgentStep struct {
	TaskID string     `json:"task_id"`
	Result StepResult `json:"result"`
}
type AgentMessage struct {
	Step         *AgentStep `json:"step,omitempty"`
	Log          *AgentLog  `json:"log,omitempty"`
	Kind         string     `json:"kind"`
	Task         *Task      `json:"task,omitempty"`
	Result       *Result    `json:"result,omitempty"`
	Version      string     `json:"version,omitempty"`
	Capabilities []string   `json:"capabilities,omitempty"`
}

func Wire(m AgentMessage) *structpb.Struct {
	var d map[string]any
	json.Unmarshal(raw(m), &d)
	v, _ := structpb.NewStruct(d)
	return v
}
func Unwire(v *structpb.Struct) AgentMessage {
	var m AgentMessage
	json.Unmarshal(raw(v.AsMap()), &m)
	return m
}

type agentConnection struct {
	send    chan AgentMessage
	pending map[string]chan Result
	mu      sync.Mutex
	done    chan struct{}
}
type Gateway struct {
	DB          *pgxpool.Pool
	mu          sync.Mutex
	connections map[string]*agentConnection
}

func NewGateway(db *pgxpool.Pool) *Gateway {
	return &Gateway{DB: db, connections: map[string]*agentConnection{}}
}

type gatewayServer interface{ Connect(grpc.ServerStream) error }

func (g *Gateway) Connect(stream grpc.ServerStream) error {
	p, ok := peer.FromContext(stream.Context())
	if !ok {
		return errors.New("TLS required")
	}
	info, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(info.State.VerifiedChains) == 0 {
		return errors.New("mTLS required")
	}
	cert := info.State.PeerCertificates[0]
	id := cert.Subject.CommonName
	serial := cert.SerialNumber.String()
	var state, dbSerial string
	if e := g.DB.QueryRow(stream.Context(), `SELECT a.state,a.certificate_serial FROM flow_agent a JOIN cmdb_entity c ON c.item_uid=a.asset_id WHERE a.id::text=$1 AND c.lifecycle_state<>'maintained' AND c.maintenance_state<>'maintained'`, id).Scan(&state, &dbSerial); e != nil || state != "enrolled" || dbSerial != serial {
		return errors.New("agent not approved")
	}
	c := &agentConnection{send: make(chan AgentMessage, 8), pending: map[string]chan Result{}, done: make(chan struct{})}
	g.mu.Lock()
	if _, exists := g.connections[id]; exists {
		g.mu.Unlock()
		return errors.New("already connected")
	}
	g.connections[id] = c
	g.mu.Unlock()
	defer func() { g.mu.Lock(); delete(g.connections, id); g.mu.Unlock(); close(c.done) }()
	receiveErr := make(chan error, 1)
	go func() {
		for {
			var wire structpb.Struct
			if e := stream.RecvMsg(&wire); e != nil {
				receiveErr <- e
				return
			}
			m := Unwire(&wire)
			var active bool
			e := g.DB.QueryRow(stream.Context(), `SELECT a.state='enrolled' AND a.certificate_serial=$2 AND a.certificate_expires_at>now() AND c.lifecycle_state<>'maintained' AND c.maintenance_state<>'maintained' FROM flow_agent a JOIN cmdb_entity c ON c.item_uid=a.asset_id WHERE a.id::text=$1`, id, serial).Scan(&active)
			if e != nil || !active {
				receiveErr <- errors.New("agent revoked")
				return
			}
			if m.Kind == "heartbeat" {
				g.DB.Exec(stream.Context(), `UPDATE flow_agent SET last_heartbeat=now(),version=$2,capabilities=$3 WHERE id::text=$1`, id, m.Version, raw(m.Capabilities))
			}
			if m.Log != nil && len(m.Log.Chunk) <= 65536 && (m.Log.Stream == "stdout" || m.Log.Stream == "stderr") {
				parts := strings.SplitN(m.Log.TaskID, ":", 2)
				if len(parts) == 2 {
					_, err := g.DB.Exec(stream.Context(), `WITH valid AS(SELECT execution_id,asset_id FROM flow_agent_task WHERE agent_id::text=$1 AND execution_id::text=$2 AND asset_id=$3), changed AS(UPDATE flow_execution_target t SET stdout=CASE WHEN $4='stdout' THEN left(t.stdout||$5,1048576) ELSE t.stdout END,stderr=CASE WHEN $4='stderr' THEN left(t.stderr||$5,1048576) ELSE t.stderr END FROM valid v WHERE t.execution_id=v.execution_id AND t.asset_id=v.asset_id AND t.state IN('RUNNING','RECONCILING') RETURNING t.execution_id,t.asset_id) UPDATE flow_agent_task a SET state='RUNNING',updated_at=now() FROM changed c WHERE a.execution_id=c.execution_id AND a.asset_id=c.asset_id`, id, parts[0], parts[1], m.Log.Stream, redactOutput(m.Log.Chunk))
					if err != nil {
						receiveErr <- err
						return
					}
				}
			}
			if m.Step != nil {
				parts := strings.SplitN(m.Step.TaskID, ":", 2)
				if len(parts) == 2 {
					var valid bool
					g.DB.QueryRow(stream.Context(), `SELECT EXISTS(SELECT 1 FROM flow_agent_task WHERE agent_id::text=$1 AND execution_id::text=$2 AND asset_id=$3)`, id, parts[0], parts[1]).Scan(&valid)
					if valid {
						(&Service{DB: g.DB}).SaveStepResults(stream.Context(), parts[0], parts[1], []StepResult{m.Step.Result})
					}
				}
			}
			if m.Result != nil {
				parts := strings.SplitN(m.Result.ID, ":", 2)
				if len(parts) == 2 && len(m.Result.Steps) > 0 {
					(&Service{DB: g.DB}).SaveStepResults(stream.Context(), parts[0], parts[1], m.Result.Steps)
				}
				if len(parts) == 2 && contains([]string{"SUCCESS", "FAILED", "RECONCILING"}, m.Result.State) {
					g.DB.Exec(stream.Context(), `WITH valid AS(SELECT t.execution_id,t.asset_id FROM flow_agent_task t WHERE t.agent_id::text=$1 AND t.execution_id::text=$2 AND t.asset_id=$3), changed AS(UPDATE flow_execution_target t SET state=$4,exit_code=$5,stdout=$6,stderr=$7,ended_at=now() FROM valid v WHERE t.execution_id=v.execution_id AND t.asset_id=v.asset_id AND t.state IN('RUNNING','RECONCILING') RETURNING t.execution_id,t.asset_id) UPDATE flow_agent_task a SET state=$4,result=$8,updated_at=now() FROM changed c WHERE a.execution_id=c.execution_id AND a.asset_id=c.asset_id`, id, parts[0], parts[1], m.Result.State, m.Result.Exit, redactOutput(m.Result.Stdout), redactOutput(m.Result.Stderr), raw(m.Result))
				}
				c.mu.Lock()
				ch := c.pending[m.Result.ID]
				c.mu.Unlock()
				if ch != nil {
					select {
					case ch <- *m.Result:
					default:
					}
				}
			}
		}
	}()
	for {
		select {
		case e := <-receiveErr:
			return e
		case <-stream.Context().Done():
			return stream.Context().Err()
		case message := <-c.send:
			if e := stream.SendMsg(Wire(message)); e != nil {
				return e
			}
		}
	}
}
func (g *Gateway) Execute(ctx context.Context, id string, task Task) (Result, error) {
	if g == nil {
		return Result{}, problem(503, "AGENT_GATEWAY_NOT_RUNNING")
	}
	g.mu.Lock()
	c := g.connections[id]
	g.mu.Unlock()
	if c == nil {
		return Result{}, problem(409, "AGENT_OFFLINE")
	}
	ch := make(chan Result, 1)
	c.mu.Lock()
	c.pending[task.ID] = ch
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.pending, task.ID); c.mu.Unlock() }()
	select {
	case c.send <- AgentMessage{Kind: "task", Task: &task}:
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
	select {
	case r := <-ch:
		return r, nil
	case <-ctx.Done():
		select {
		case c.send <- AgentMessage{Kind: "cancel", Task: &task}:
		default:
		}
		return Result{ID: task.ID, State: "RECONCILING", Exit: -1, Stderr: "Task outcome unknown; no SSH fallback"}, nil
	case <-c.done:
		return Result{ID: task.ID, State: "RECONCILING", Exit: -1, Stderr: "Agent disconnected after dispatch; reconcile original task"}, nil
	}
}
func (g *Gateway) Serve(ctx context.Context, address string, cert tls.Certificate, ca *x509.CertPool) error {
	listener, e := net.Listen("tcp", address)
	if e != nil {
		return e
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}, ClientCAs: ca, ClientAuth: tls.RequireAndVerifyClientCert})), grpc.MaxRecvMsgSize(3<<20))
	server.RegisterService(&grpc.ServiceDesc{ServiceName: "keel.agent.v1.Gateway", HandlerType: (*gatewayServer)(nil), Streams: []grpc.StreamDesc{{StreamName: "Connect", Handler: func(s any, stream grpc.ServerStream) error { return s.(gatewayServer).Connect(stream) }, ServerStreams: true, ClientStreams: true}}}, g)
	go func() { <-ctx.Done(); server.Stop() }()
	return server.Serve(listener)
}

// Cancel only the exact accepted task; lack of a live connection is not proof of termination.
func (g *Gateway) Cancel(agent, task string) {
	if g == nil {
		return
	}
	g.mu.Lock()
	c := g.connections[agent]
	g.mu.Unlock()
	if c == nil {
		return
	}
	select {
	case c.send <- AgentMessage{Kind: "cancel", Task: &Task{ID: task}}:
	default:
	}
}
