package flows

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Opt-in and isolated: never point this at the current development/production database.
func TestKestraSSHIntegration(t *testing.T) {
	dsn := os.Getenv("FLOWS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated integration DB not configured")
	}
	if !strings.Contains(dsn, "keel_flows_dev") {
		t.Fatal("requires named isolated DB")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var user string
	if e = db.QueryRow(ctx, `SELECT id::text FROM app_user WHERE username='admin'`).Scan(&user); e != nil {
		t.Fatal(e)
	}
	scope := Scope{User: user, Actor: "synthetic-local-acceptance", Projects: []string{"proj-flows-synthetic"}}
	service := &Service{DB: db, Engine: NewKestra()}
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	priv, e := x509.MarshalECPrivateKey(key)
	if e != nil {
		t.Fatal(e)
	}
	private := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: priv})
	signer, e := ssh.ParsePrivateKey(private)
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	t.Setenv("KEEL_SSH_SECRET_DIR", dir)
	if e = os.WriteFile(filepath.Join(dir, "key.pem"), private, 0600); e != nil {
		t.Fatal(e)
	}
	var firstCalls, secondCalls atomic.Int32
	first := testSSH(t, signer, &firstCalls, false)
	second := testSSH(t, signer, &secondCalls, true)
	known := knownhosts.Line([]string{first.Addr().String()}, signer.PublicKey()) + "\n" + knownhosts.Line([]string{second.Addr().String()}, signer.PublicKey()) + "\n"
	os.WriteFile(filepath.Join(dir, "known_hosts"), []byte(known), 0600)
	ids := []string{}
	for i, server := range []net.Listener{first, second} {
		id := "synthetic-flow-target-" + uuid.NewString()
		ids = append(ids, id)
		host, portText, _ := net.SplitHostPort(server.Addr().String())
		var port int
		fmt.Sscan(portText, &port)
		_, e = db.Exec(ctx, `INSERT INTO cmdb_entity(item_uid,payload,asset_type,name,source_type,source_ref) VALUES($1,$2,'server',$3,'manual','synthetic-flows-acceptance')`, id, raw(map[string]any{"is_demo": true, "os": "Linux", "management_ip": host}), fmt.Sprint("Synthetic local SSH ", i))
		if e != nil {
			t.Fatal(e)
		}
		_, e = db.Exec(ctx, `INSERT INTO cmdb_asset_scope(project_id,asset_id,source_ref) VALUES($1,$2,'synthetic-flows-acceptance')`, scope.Projects[0], id)
		if e != nil {
			t.Fatal(e)
		}
		_, e = db.Exec(ctx, `INSERT INTO flow_ssh_trust(id,project_id,asset_id,host,port,username,secret_ref,known_hosts_ref,approved_by,status) VALUES(gen_random_uuid(),$1,$2,$3,$4,'synthetic','key.pem','known_hosts',$5,'approved')`, scope.Projects[0], id, host, port, user)
		if e != nil {
			t.Fatal(e)
		}
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if strings.HasSuffix(r.URL.Path, "/results") {
			var executionID string
			if e := db.QueryRow(r.Context(), `SELECT execution_id::text FROM flow_dispatch_ticket WHERE hash=$1 AND expires_at>now()`, hash(token)).Scan(&executionID); e != nil {
				w.WriteHeader(403)
				return
			}
			ex, e := service.Execution(r.Context(), scope, executionID)
			if e != nil {
				w.WriteHeader(500)
				return
			}
			json.NewEncoder(w).Encode(ex.Targets)
			return
		}
		var input struct {
			Asset    string `json:"asset_id"`
			Required string `json:"required_transport"`
		}
		json.NewDecoder(r.Body).Decode(&input)
		result, e := service.Dispatch(r.Context(), token, input.Asset, input.Required)
		if e != nil {
			w.WriteHeader(422)
			json.NewEncoder(w).Encode(map[string]string{"error": e.Error()})
			return
		}
		if result.State != "SUCCESS" {
			w.WriteHeader(422)
		}
		json.NewEncoder(w).Encode(result)
	}))
	server.Listener.Close()
	server.Listener, e = net.Listen("tcp", os.Getenv("FLOWS_TEST_CALLBACK_HOST")+":0")
	if e != nil {
		t.Fatal(e)
	}
	server.Start()
	defer server.Close()
	t.Setenv("KEEL_WORKER_CALLBACK_URL", server.URL)
	run := RunRequest{Project: scope.Projects[0], Targets: ids, Kind: "bash", Command: "printf synthetic-controlled-target", Policy: "ssh_only", Timeout: 15, Concurrency: 2, Source: "synthetic-local-acceptance"}
	ex, e := service.Start(ctx, scope, run, "integration-"+uuid.NewString())
	if e != nil {
		t.Fatal(e)
	}
	ex = awaitExecution(t, ctx, service, scope, ex.ID)
	if firstCalls.Load() != 1 || secondCalls.Load() != 1 {
		t.Fatalf("unexpected calls %d/%d", firstCalls.Load(), secondCalls.Load())
	}
	success, failed := 0, 0
	for _, target := range ex.Targets {
		if target.State == "SUCCESS" {
			success++
		}
		if target.State == "FAILED" {
			failed++
		}
		if target.Actual == nil || *target.Actual != "SSH" {
			t.Fatal("actual transport missing")
		}
	}
	if success != 1 || failed != 1 {
		t.Fatalf("expected partial target result, got %d/%d", success, failed)
	}
	retry, e := service.Retry(ctx, scope, ex.ID, "retry-"+uuid.NewString())
	if e != nil {
		t.Fatal(e)
	}
	retry = awaitExecution(t, ctx, service, scope, retry.ID)
	if current, _ := retry.Engine["state"].(map[string]any); fmt.Sprint(current["current"]) != "SUCCESS" {
		t.Fatalf("retry engine state not successful: %v", current)
	}
	if len(retry.Targets) != 1 || retry.Targets[0].ID != ids[1] || retry.Targets[0].State != "SUCCESS" || firstCalls.Load() != 1 || secondCalls.Load() != 2 {
		t.Fatal("failed-target retry reran wrong targets")
	}
	again, e := service.Start(ctx, scope, run, "idempotent-"+ex.ID)
	if e != nil {
		t.Fatal(e)
	}
	duplicate, e := service.Start(ctx, scope, run, "idempotent-"+ex.ID)
	if e != nil || again.ID != duplicate.ID {
		t.Fatal("idempotency broken")
	}
	awaitExecution(t, ctx, service, scope, again.ID)
	t.Log("Real Kestra + isolated PostgreSQL + real SSH protocol to synthetic endpoints: partial failure, new failed-only execution, idempotency, strict known_hosts; no external devices")
}
func awaitExecution(t *testing.T, ctx context.Context, s *Service, p Scope, id string) Execution {
	t.Helper()
	for {
		ex, e := s.Execution(ctx, p, id)
		if e != nil {
			t.Fatal(e)
		}
		state, _ := ex.Engine["state"].(map[string]any)
		if contains([]string{"SUCCESS", "FAILED", "WARNING", "KILLED"}, fmt.Sprint(state["current"])) {
			return ex
		}
		select {
		case <-ctx.Done():
			t.Fatal("engine timed out")
		case <-time.After(300 * time.Millisecond):
		}
	}
}
func testSSH(t *testing.T, signer ssh.Signer, calls *atomic.Int32, failFirst bool) net.Listener {
	t.Helper()
	listener, e := net.Listen("tcp", "0.0.0.0:0")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { listener.Close() })
	config := &ssh.ServerConfig{PublicKeyCallback: func(meta ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if string(key.Marshal()) != string(signer.PublicKey().Marshal()) {
			return nil, fmt.Errorf("key mismatch")
		}
		return nil, nil
	}}
	config.AddHostKey(signer)
	go func() {
		for {
			conn, e := listener.Accept()
			if e != nil {
				return
			}
			go func() {
				server, chans, requests, e := ssh.NewServerConn(conn, config)
				if e != nil {
					conn.Close()
					return
				}
				defer server.Close()
				go ssh.DiscardRequests(requests)
				for newChannel := range chans {
					channel, requests, e := newChannel.Accept()
					if e != nil {
						return
					}
					go func() {
						defer channel.Close()
						for request := range requests {
							if request.Type != "exec" {
								request.Reply(false, nil)
								continue
							}
							request.Reply(true, nil)
							n := calls.Add(1)
							code := uint32(0)
							if failFirst && n == 1 {
								code = 7
							}
							fmt.Fprint(channel, "synthetic SSH target output\n")
							channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{code}))
							return
						}
					}()
				}
			}()
		}
	}()
	return listener
}
