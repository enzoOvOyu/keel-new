// Keel target agent. It executes authorized individual tasks, never schedules batches.
package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/structpb"
	"io"
	"keel.local/dev/internal/flows"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const version = "0.2.0"

var caps = []string{"job.pipeline", "exec.command", "script.bash", "script.python", "task.cancel", "task.stream", "inventory.collect", "file.upload", "file.download", "process.control"}
var taskRegistry = struct {
	sync.Mutex
	active map[string]context.CancelFunc
}{active: map[string]context.CancelFunc{}}
var outbound = struct {
	sync.Mutex
	send chan flows.AgentMessage
	ctx  context.Context
}{}

func emitAgent(message flows.AgentMessage) {
	outbound.Lock()
	send, ctx := outbound.send, outbound.ctx
	outbound.Unlock()
	if send == nil || ctx == nil {
		return
	}
	select {
	case send <- message:
	case <-ctx.Done():
	}
}

func main() {
	if os.Getenv("KEEL_AGENT_ALLOWED_SERVICES") != "" {
		caps = append(caps, "service.control")
	}
	gateway := flag.String("gateway", "", "gRPC TLS address")
	api := flag.String("enroll-url", "", "HTTPS Keel API URL")
	tokenFile := flag.String("token-file", "", "one-use token file, removed after enrollment")
	state := flag.String("state-dir", "/var/lib/keel-agent", "private persistent directory")
	flag.Parse()
	if os.Geteuid() == 0 {
		fatal("run agent as an unprivileged service user")
	}
	if *gateway == "" {
		fatal("gateway required")
	}
	if e := os.MkdirAll(*state, 0700); e != nil {
		fatal("state unavailable")
	}
	os.Chmod(*state, 0700)
	if *tokenFile != "" {
		if _, e := os.Stat(*tokenFile); os.IsNotExist(e) {
			*tokenFile = ""
		}
	}
	if *tokenFile != "" {
		if e := enroll(*api, *tokenFile, *state); e != nil {
			fatal("enrollment failed: " + e.Error())
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	for ctx.Err() == nil {
		if e := connect(ctx, *gateway, *state); e != nil {
			fmt.Fprintln(os.Stderr, "gateway disconnected; reconnecting without re-executing tasks")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}
func fatal(v string) { fmt.Fprintln(os.Stderr, v); os.Exit(1) }
func enroll(api, tokenFile, dir string) error {
	if !strings.HasPrefix(api, "https://") {
		return fmt.Errorf("HTTPS required for token exchange")
	}
	token, e := os.ReadFile(tokenFile)
	if e != nil {
		return e
	}
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		return e
	}
	csr, e := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: "enrollment"}}, key)
	if e != nil {
		return e
	}
	identity := map[string]string{}
	for field, path := range map[string]string{"machine_id": "/etc/machine-id", "dmi_uuid": "/sys/class/dmi/id/product_uuid", "serial": "/sys/class/dmi/id/product_serial"} {
		b, _ := os.ReadFile(path)
		identity[field] = strings.TrimSpace(string(b))
	}
	identity["hostname"], _ = os.Hostname()
	input := flows.EnrollInput{Token: strings.TrimSpace(string(token)), CSR: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csr})), Identity: identity, Version: version, Capabilities: caps}
	body, _ := json.Marshal(input)
	client := &http.Client{Timeout: 20 * time.Second}
	res, e := client.Post(strings.TrimRight(api, "/")+"/api/v1/flows/agents/register", "application/json", bytes.NewReader(body))
	if e != nil {
		return e
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		return fmt.Errorf("HTTP %d", res.StatusCode)
	}
	var envelope struct {
		Data struct{ ID, State, Certificate, CA string }
	}
	if e = json.NewDecoder(res.Body).Decode(&envelope); e != nil {
		return e
	}
	priv, _ := x509.MarshalPKCS8PrivateKey(key)
	for name, value := range map[string][]byte{"key.pem": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: priv}), "cert.pem": []byte(envelope.Data.Certificate), "ca.pem": []byte(envelope.Data.CA)} {
		if e = os.WriteFile(filepath.Join(dir, name), value, 0600); e != nil {
			return e
		}
	}
	os.Remove(tokenFile)
	fmt.Println("enrolled", envelope.Data.ID, envelope.Data.State)
	return nil
}

type limitWriter struct {
	emit func(string)
	b    bytes.Buffer
	mu   sync.Mutex
}

func (w *limitWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := len(p)
	space := (1 << 20) - w.b.Len()
	if space > 0 {
		if len(p) > space {
			p = p[:space]
		}
		w.b.Write(p)
		if w.emit != nil {
			w.emit(string(p))
		}
	}
	return n, nil
}
func connect(ctx context.Context, address, dir string) error {
	cert, e := tls.LoadX509KeyPair(filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem"))
	if e != nil {
		return e
	}
	ca, e := os.ReadFile(filepath.Join(dir, "ca.pem"))
	if e != nil {
		return e
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return fmt.Errorf("invalid CA")
	}
	conn, e := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}, RootCAs: pool})))
	if e != nil {
		return e
	}
	defer conn.Close()
	stream, e := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true, ClientStreams: true}, flows.GatewayMethod)
	if e != nil {
		return e
	}
	send := make(chan flows.AgentMessage, 32)
	sendErr := make(chan error, 1)
	streamCtx, cancel := context.WithCancel(ctx)
	outbound.Lock()
	outbound.send = send
	outbound.ctx = streamCtx
	outbound.Unlock()
	defer cancel()
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-streamCtx.Done():
				return
			case m := <-send:
				if e := stream.SendMsg(flows.Wire(m)); e != nil {
					sendErr <- e
					return
				}
			case <-ticker.C:
				send <- flows.AgentMessage{Kind: "heartbeat", Version: version, Capabilities: caps}
			}
		}
	}()
	send <- flows.AgentMessage{Kind: "heartbeat", Version: version, Capabilities: caps}
	entries, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	for _, entry := range entries {
		data, err := os.ReadFile(entry)
		if err == nil {
			var result flows.Result
			if json.Unmarshal(data, &result) == nil && result.ID != "" {
				send <- flows.AgentMessage{Kind: "result", Result: &result}
			}
		}
	}
	active := taskRegistry.active
	mu := &taskRegistry.Mutex
	for {
		var wire structpb.Struct
		if e = stream.RecvMsg(&wire); e != nil {
			return e
		}
		m := flows.Unwire(&wire)
		if m.Task == nil {
			continue
		}
		task := *m.Task
		if m.Kind == "query" {
			var result flows.Result
			data, err := os.ReadFile(filepath.Join(dir, flows.Digest([]byte(task.ID))+".json"))
			if err != nil || json.Unmarshal(data, &result) != nil {
				result = flows.Result{ID: task.ID, State: "RECONCILING", Exit: -1, Stderr: "No durable outcome; absence is not proof of no execution"}
			}
			emitAgent(flows.AgentMessage{Kind: "result", Result: &result})
			continue
		}
		if m.Kind == "cancel" {
			mu.Lock()
			if c := active[task.ID]; c != nil {
				c()
			}
			mu.Unlock()
			continue
		}
		if m.Kind != "task" {
			continue
		}
		mu.Lock()
		if active[task.ID] != nil {
			mu.Unlock()
			continue
		}
		taskCtx, stop := context.WithCancel(ctx)
		active[task.ID] = stop
		mu.Unlock()
		go func() {
			defer stop()
			result := execute(taskCtx, dir, task, func(name, chunk string) {
				emitAgent(flows.AgentMessage{Kind: "log", Log: &flows.AgentLog{TaskID: task.ID, Stream: name, Chunk: chunk}})
			})
			mu.Lock()
			delete(active, task.ID)
			mu.Unlock()
			emitAgent(flows.AgentMessage{Kind: "result", Result: &result})
		}()
	}
}
func execute(ctx context.Context, dir string, t flows.Task, callbacks ...func(string, string)) flows.Result {
	out := flows.Result{ID: t.ID, State: "FAILED", Exit: -1}
	if t.SHA256 != flows.TaskDigest(t) {
		out.Stderr = "Script digest mismatch"
		return out
	}
	journal := filepath.Join(dir, flows.Digest([]byte(t.ID))+".json")
	if b, e := os.ReadFile(journal); e == nil {
		json.Unmarshal(b, &out)
		if out.Digest != t.SHA256 {
			return flows.Result{ID: t.ID, State: "FAILED", Exit: -1, Stderr: "Task ID reused with different content"}
		}
		return out
	}
	out.State = "RECONCILING"
	out.Digest = t.SHA256
	if e := os.WriteFile(journal, mustJSON(out), 0600); e != nil {
		return out
	}
	if len(t.Steps) > 0 {
		ctx, cancel := context.WithTimeout(ctx, time.Duration(t.Timeout)*time.Second)
		defer cancel()
		plan, err := flows.PlanSteps(t.Steps)
		if err != nil {
			out.State = "FAILED"
			out.Stderr = err.Error()
			return out
		}
		out.State = "SUCCESS"
		out.Exit = 0
		values := map[string]flows.StepResult{}
		for _, step := range plan {
			v := flows.StepResult{ID: step.ID, State: "BLOCKED", Exit: -1}
			if out.State == "SUCCESS" {
				emitAgent(flows.AgentMessage{Kind: "step", Step: &flows.AgentStep{TaskID: t.ID, Result: flows.StepResult{ID: step.ID, State: "RUNNING", Exit: -1}}})
				child := flows.StepTask(t, step, values)
				result := execute(ctx, dir, child, callbacks...)
				v = flows.StepResult{ID: step.ID, State: result.State, Exit: result.Exit, Stdout: flows.BoundStepOutput(result.Stdout), Stderr: flows.BoundStepOutput(result.Stderr)}
				out.State = result.State
				out.Exit = result.Exit
				out.Stdout += v.Stdout
				out.Stderr += v.Stderr
			}
			out.Steps = append(out.Steps, v)
			values[step.ID] = v
			emitAgent(flows.AgentMessage{Kind: "step", Step: &flows.AgentStep{TaskID: t.ID, Result: v}})
		}
		os.WriteFile(journal, mustJSON(out), 0600)
		return out
	}
	if t.Kind == "process.control" || t.Kind == "service.control" {
		out = controlTask(ctx, dir, t)
		out.Digest = t.SHA256
		os.WriteFile(journal, mustJSON(out), 0600)
		return out
	}
	if t.Kind == "file.upload" || t.Kind == "file.download" {
		out = fileTask(dir, t)
		out.Digest = t.SHA256
		os.WriteFile(journal, mustJSON(out), 0600)
		return out
	}
	if t.Kind == "inventory.collect" {
		t.Kind = "command"
		t.Command = "uname -srm; hostname; if command -v nvidia-smi >/dev/null 2>&1; then nvidia-smi --query-gpu=name,uuid --format=csv,noheader; else printf GPU_UNSUPPORTED; fi"
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(t.Timeout)*time.Second)
	defer cancel()
	shell, args := "sh", []string{"-s"}
	if t.Kind == "bash" {
		shell = "bash"
	}
	if t.Kind == "python" {
		shell = "python3"
		args = []string{"-"}
	}
	if !strings.Contains(" command bash python ", " "+t.Kind+" ") {
		out.State = "FAILED"
		out.Stderr = "Unsupported task kind"
		return out
	}
	command := exec.CommandContext(ctx, shell, args...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	command.Cancel = func() error { return syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }
	command.WaitDelay = 2 * time.Second
	command.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "LANG=C.UTF-8"}
	for key, value := range t.Env {
		command.Env = append(command.Env, key+"="+value)
	}
	command.Dir = dir
	command.Stdin = strings.NewReader(t.Command)
	stdout, stderr := &limitWriter{}, &limitWriter{}
	if len(callbacks) > 0 {
		stdout.emit = func(chunk string) { callbacks[0]("stdout", chunk) }
		stderr.emit = func(chunk string) { callbacks[0]("stderr", chunk) }
	}
	command.Stdout = stdout
	command.Stderr = stderr
	err := command.Run()
	out.State = "SUCCESS"
	out.Exit = 0
	if err != nil {
		out.State = "FAILED"
		out.Exit = -1
		if ex, ok := err.(*exec.ExitError); ok {
			out.Exit = ex.ExitCode()
		}
	}
	out.Stdout = stdout.b.String()
	out.Stderr = stderr.b.String()
	os.WriteFile(journal, mustJSON(out), 0600)
	return out
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

var _ io.Writer = (*limitWriter)(nil)
