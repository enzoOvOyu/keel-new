package flows

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestWorkspaceQueryIntegration(t *testing.T) {
	dsn := os.Getenv("FLOWS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated integration DB not configured")
	}
	if !strings.Contains(dsn, "keel_flows_dev") {
		t.Fatal("isolated DB name required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var user, project string
	if e = db.QueryRow(ctx, `SELECT id::text FROM app_user WHERE username='admin'`).Scan(&user); e != nil {
		t.Fatal(e)
	}
	if e = db.QueryRow(ctx, `SELECT id FROM project ORDER BY id LIMIT 1`).Scan(&project); e != nil {
		t.Fatal(e)
	}
	fid, first, second := uuid.NewString(), uuid.NewString(), uuid.NewString()
	ns := Namespace(project)
	name := "synthetic-workspace-" + fid
	source := "id: test\nnamespace: " + ns + "\ninputs:\n  - id: arbitrary\n    type: SECRET\ntasks:\n  - id: log\n    type: io.kestra.plugin.core.log.Log\n    message: synthetic\n"
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/main/metrics/"):
			json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"name": "synthetic_count", "value": 1}}, "total": 1})
		case strings.HasPrefix(r.URL.Path, "/api/v1/main/executions/"):
			json.NewEncoder(w).Encode(map[string]any{"id": "synthetic-engine", "namespace": ns, "flowId": "test", "flowRevision": 2, "state": map[string]any{"current": "FAILED"}, "inputs": map[string]any{"arbitrary": "never-expose", "ticket": "never-expose", "normal": "synthetic"}, "outputs": map[string]any{"password": "never-expose"}})
		case strings.Contains(r.URL.Path, "/flows/"+ns+"/"):
			json.NewEncoder(w).Encode(map[string]any{"id": "test", "namespace": ns, "revision": 2, "source": source})
		default:
			json.NewEncoder(w).Encode([]any{map[string]any{"id": "test", "revision": 2, "disabled": false}})
		}
	}))
	defer engine.Close()
	s := &Service{DB: db, Engine: &Kestra{Base: engine.URL, Tenant: "main", HTTP: engine.Client()}}
	scope := Scope{User: user, Actor: "synthetic-workspace", Projects: []string{project}}
	if _, e = db.Exec(ctx, `INSERT INTO flow_binding(id,project_id,namespace,flow_id,name,target_type,created_by) VALUES($1,$2,$3,'test',$4,'server',$5)`, fid, project, ns, name, user); e != nil {
		t.Fatal(e)
	}
	defer db.Exec(context.Background(), `DELETE FROM flow_binding WHERE id=$1`, fid)
	for i, id := range []string{first, second} {
		var parent any
		if i == 1 {
			parent = first
		}
		if _, e = db.Exec(ctx, `INSERT INTO flow_execution_binding(id,project_id,flow_id,kestra_id,namespace,engine_flow_id,flow_revision,source_module,requester_id,idempotency_key,request_hash,request,parent_id,created_at,engine_state) VALUES($1,$2,$3,$4,$5,'test',2,'flows',$6,$9,'synthetic','{}',$7,$8,'FAILED')`, id, project, fid, "synthetic-"+id, ns, user, parent, time.Now().Add(time.Duration(i-2)*time.Minute), "synthetic-"+id); e != nil {
			t.Fatal(e)
		}
	}
	defer db.Exec(context.Background(), `DELETE FROM flow_execution_binding WHERE id=ANY($1::uuid[])`, []string{first, second})
	defer db.Exec(context.Background(), `DELETE FROM flow_event WHERE execution_id=ANY($1::uuid[])`, []string{first, second})
	if e = s.RecordEvent(ctx, second, "log", map[string]any{"taskId": "log", "level": "ERROR", "timestamp": time.Now().Format(time.RFC3339), "message": "synthetic problem"}); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, `INSERT INTO flow_audit(id,project_id,actor,action,resource_id,details) VALUES(gen_random_uuid(),$1,'synthetic-ui','flow.save',$2,'{}')`, project, fid); e != nil {
		t.Fatal(e)
	}
	defer db.Exec(context.Background(), `DELETE FROM flow_audit WHERE resource_id=$1`, fid)
	if _, e = db.Exec(ctx, `INSERT INTO flow_schedule(flow_id,owner_id,engine_flow_id,cron,timezone,enabled,request,revision) VALUES($1,$2,$3,'*/30 * * * *','UTC',false,'{}',1)`, fid, user, "synthetic-schedule-"+fid); e != nil {
		t.Fatal(e)
	}
	defer db.Exec(context.Background(), `DELETE FROM flow_schedule WHERE flow_id=$1`, fid)
	catalog, e := s.Catalog(ctx, scope, url.Values{"q": {name}, "limit": {"1"}})
	if e != nil || catalog.Total != 1 || len(catalog.Items) != 1 || catalog.Items[0].Recent == nil || catalog.Items[0].Recent.ID != second {
		t.Fatalf("catalog %+v %v", catalog, e)
	}
	if e = s.RecordEvent(ctx, first, "execution", map[string]any{"state": map[string]any{"current": "FAILED", "startDate": "2026-01-01T00:00:00Z", "endDate": "2026-01-01T00:00:08Z"}}); e != nil {
		t.Fatal(e)
	}
	item := catalog.Items[0]
	if len(item.RecentExecutions) != 2 || item.RecentExecutions[0].ID != second || item.UpdatedAt == nil || item.UpdatedBy == nil || *item.UpdatedBy != "synthetic-ui" || len(item.Triggers) != 1 || item.Triggers[0].Source != "scoped_schedule" || item.Triggers[0].Enabled {
		t.Fatalf("catalog prototype metadata incomplete: %+v", item)
	}
	counted, e := s.Catalog(ctx, scope, url.Values{"q": {name}, "state": {"DRAFT"}})
	if e != nil || counted.Total != 0 || counted.StateCounts["ALL"] != 1 || counted.StateCounts["ENABLED"] != 1 {
		t.Fatalf("tab counts depend on selected state: %+v %v", counted, e)
	}
	summary, e := s.ExecutionSummary(ctx, scope, url.Values{"flow_id": {fid}})
	if e != nil || summary.Total != 2 || summary.States["FAILED"] != 2 {
		t.Fatalf("summary %+v %v", summary, e)
	}
	if summary.Duration.SampleCount != 1 || summary.Duration.MissingCount != 1 || summary.Duration.Mean == nil || *summary.Duration.Mean != 8 || summary.Duration.P95 != nil || len(summary.Daily) == 0 {
		t.Fatalf("duration evidence/coverage wrong: %+v", summary)
	}

	page, e := s.ExecutionPage(ctx, scope, url.Values{"flow_id": {fid}, "revision": {"999"}})
	if e != nil || len(page.Items) != 0 {
		t.Fatalf("execution page revision filter ignored: %+v %v", page, e)
	}
	filtered, e := s.ExecutionSummary(ctx, scope, url.Values{"flow_id": {fid}, "revision": {"999"}})
	if e != nil || filtered.Total != 0 {
		t.Fatalf("revision filter ignored: %+v %v", filtered, e)
	}
	if _, e = s.ExecutionSummary(ctx, scope, url.Values{"revision": {"0"}}); e == nil {
		t.Fatal("invalid zero revision accepted")
	}

	logs, e := s.LogSearch(ctx, scope, url.Values{"flow_id": {fid}})
	if e != nil || len(logs.Items) != 1 || logs.Items[0].ExecutionID != second {
		t.Fatalf("logs %+v %v", logs, e)
	}
	insights, e := s.ExecutionInsights(ctx, scope, second, url.Values{})
	if e != nil || insights.Previous == nil || *insights.Previous != first || len(insights.Relations) != 1 || insights.MetricsTotal != 1 {
		t.Fatalf("insights %+v %v", insights, e)
	}
	variables, e := s.ExecutionVariables(ctx, scope, second)
	if e != nil {
		t.Fatal(e)
	}
	bytes, _ := json.Marshal(variables)
	if strings.Contains(string(bytes), "never-expose") {
		t.Fatal("variables leaked SECRET input or output")
	}
	replay, e := s.ReplayRequest(ctx, scope, second, "all", "synthetic-replay-nonce")
	if e != nil || replay.Project != project || replay.Flow != fid || replay.ApprovalID != "" || replay.Inputs["arbitrary"] != "" || replay.ApprovalContext != replayContext(second, "synthetic-replay-nonce") {
		t.Fatalf("replay did not freeze scope or mask secrets: %+v %v", replay, e)
	}
	denied := Scope{User: user, Projects: []string{"unauthorized-synthetic"}}
	if _, e = s.ExecutionInsights(ctx, denied, second, url.Values{}); e == nil {
		t.Fatal("cross-project execution accepted")
	}
	logs, e = s.LogSearch(ctx, denied, url.Values{"flow_id": {fid}})
	if e != nil || len(logs.Items) != 0 {
		t.Fatal("cross-project logs leaked")
	}
	// Name stays in the immutable existing request; attention is a server filter, not a browser count.
	if _, err := db.Exec(ctx, `UPDATE flow_execution_binding SET engine_state='SUCCESS',request=jsonb_build_object('name','synthetic titled run') WHERE id=$1`, first); err != nil {
		t.Fatal(err)
	}
	attention, err := s.ExecutionPage(ctx, scope, url.Values{"flow_id": {fid}, "attention": {"true"}})
	if err != nil || len(attention.Items) != 1 || attention.Items[0].ID != second {
		t.Fatalf("attention scope wrong: %+v %v", attention, err)
	}
	named, err := s.ExecutionPage(ctx, scope, url.Values{"flow_id": {fid}, "q": {"synthetic titled run"}})
	if err != nil || len(named.Items) != 1 || named.Items[0].Name == nil || *named.Items[0].Name != "synthetic titled run" {
		t.Fatalf("persisted name unavailable: %+v %v", named, err)
	}
	if _, err = s.ExecutionPage(ctx, scope, url.Values{"attention": {"maybe"}}); err == nil {
		t.Fatal("invalid attention filter accepted")
	}

}
