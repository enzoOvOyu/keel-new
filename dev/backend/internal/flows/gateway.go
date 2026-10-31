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
	"sync"
	"time"
)

const GatewayMethod = "/keel.agent.v1.Gateway/Connect"

type AgentMessage struct {
	Kind         string   `json:"kind"`
	Task         *Task    `json:"task,omitempty"`
	Result       *Result  `json:"result,omitempty"`
	Version      string   `json:"version,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
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
	if e := g.DB.QueryRow(stream.Context(), `SELECT state,certificate_serial FROM flow_agent WHERE id::text=$1`, id).Scan(&state, &dbSerial); e != nil || state != "enrolled" || dbSerial != serial {
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
			e := g.DB.QueryRow(stream.Context(), `SELECT state='enrolled' AND certificate_serial=$2 AND certificate_expires_at>now() FROM flow_agent WHERE id::text=$1`, id, serial).Scan(&active)
			if e != nil || !active {
				receiveErr <- errors.New("agent revoked")
				return
			}
			if m.Kind == "heartbeat" {
				g.DB.Exec(stream.Context(), `UPDATE flow_agent SET last_heartbeat=now(),version=$2,capabilities=$3 WHERE id::text=$1`, id, m.Version, raw(m.Capabilities))
			}
			if m.Result != nil {
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

var _ = time.Second
