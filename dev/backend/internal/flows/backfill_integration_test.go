package flows

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBackfillNeverReusesScheduleApproval(t *testing.T) {
	dsn := os.Getenv("FLOWS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated integration DB not configured")
	}
	if !strings.Contains(dsn, "keel_flows_dev") {
		t.Fatal("isolated DB required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var project, user string
	if e = db.QueryRow(ctx, `SELECT id FROM project ORDER BY id LIMIT 1`).Scan(&project); e != nil {
		t.Fatal(e)
	}
	if e = db.QueryRow(ctx, `SELECT id::text FROM app_user WHERE username='admin'`).Scan(&user); e != nil {
		t.Fatal(e)
	}
	asset := "synthetic-backfill-" + uuid.NewString()
	agent, fid := uuid.NewString(), uuid.NewString()
	ns := Namespace(project)
	if _, e = db.Exec(ctx, `INSERT INTO cmdb_entity(item_uid,payload,asset_type,name,source_type,source_ref,maintenance_state,lifecycle_state) VALUES($1,'{"os":"Linux"}','server','Synthetic backfill','manual','synthetic-backfill','active','discovered')`, asset); e != nil {
		t.Fatal(e)
	}
	defer db.Exec(context.Background(), `DELETE FROM cmdb_entity WHERE item_uid=$1`, asset)
	if _, e = db.Exec(ctx, `INSERT INTO cmdb_asset_scope(project_id,asset_id,source_ref) VALUES($1,$2,'synthetic-backfill')`, project, asset); e != nil {
		t.Fatal(e)
	}
	defer db.Exec(context.Background(), `DELETE FROM cmdb_asset_scope WHERE project_id=$1 AND asset_id=$2`, project, asset)
	if _, e = db.Exec(ctx, `INSERT INTO flow_agent(id,project_id,asset_id,identity,version,capabilities,state,last_heartbeat,certificate_serial,certificate_expires_at) VALUES($1,$2,$3,'{}','0.2.0','["script.bash","exec.command","job.pipeline"]','enrolled',now(),'synthetic',now()+interval '1 day')`, agent, project, asset); e != nil {
		t.Fatal(e)
	}
	defer db.Exec(context.Background(), `DELETE FROM flow_agent WHERE id=$1`, agent)
	if _, e = db.Exec(ctx, `INSERT INTO flow_binding(id,project_id,namespace,flow_id,name,target_type,created_by,revision) VALUES($1,$2,$3,$4,'Synthetic backfill','server',$5,2)`, fid, project, ns, "test-"+fid, user); e != nil {
		t.Fatal(e)
	}
	defer db.Exec(context.Background(), `DELETE FROM flow_binding WHERE id=$1`, fid)
	run := RunRequest{Project: project, Flow: fid, Revision: 2, Targets: []string{asset}, Kind: "bash", Command: "reboot", Policy: "agent_only", Timeout: 60, Concurrency: 1, ApprovalAction: "reinstall", ApprovalID: "synthetic-old-approval"}
	if _, e = db.Exec(ctx, `INSERT INTO flow_schedule(flow_id,owner_id,engine_flow_id,cron,timezone,enabled,request,revision) VALUES($1,$2,$3,'0 0 * * *','UTC',false,$4,1)`, fid, user, "schedule-"+fid, raw(run)); e != nil {
		t.Fatal(e)
	}
	defer db.Exec(context.Background(), `DELETE FROM flow_schedule WHERE flow_id=$1`, fid)
	var writes atomic.Int32
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writes.Add(1)
		}
		if strings.Contains(r.URL.Path, "/triggers/") {
			if !strings.HasSuffix(r.URL.Path, "/"+ns+"/schedule-"+fid) {
				t.Error("wrong scoped schedule runtime", r.URL.Path)
			}
			json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"triggerId": "cron", "nextEvaluationDate": "2026-12-01T00:00:00Z", "lastTriggeredDate": "2026-01-01T00:00:00Z", "disabled": true, "sourceDisabled": false, "locked": false}}})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"id": "test-" + fid, "namespace": ns, "revision": 2, "source": "id: test\nnamespace: " + ns + "\ninputs:\n  - id: ticket\n    type: STRING\ntasks:\n  - id: log\n    type: io.kestra.plugin.core.log.Log\n    message: synthetic\n"})
	}))
	defer engine.Close()
	s := &Service{DB: db, Engine: &Kestra{Base: engine.URL, Tenant: "main", HTTP: engine.Client()}}
	scope := Scope{User: user, Actor: "synthetic-backfill", Projects: []string{project}}
	status, e := s.ScheduleStatus(ctx, scope, fid)
	if e != nil || !status.Configured || len(status.Runtime) != 1 || status.Runtime[0].NextEvaluationDate == nil || status.Runtime[0].NextEvaluationDate.Year() != 2026 || !status.Runtime[0].Disabled {
		t.Fatalf("actual trigger runtime missing: %+v %v", status, e)
	}
	if _, e = s.ScheduleStatus(ctx, Scope{Projects: []string{"synthetic-denied"}}, fid); e == nil {
		t.Fatal("unauthorized schedule runtime exposed")
	}
	input := BackfillRequest{Start: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), ScheduleRevision: 1, FlowRevision: 2}
	preview, e := s.BackfillPreview(ctx, scope, fid, input)
	if e != nil || preview.Count != 2 || !preview.RequiresApproval || preview.Run.ApprovalID != "" {
		t.Fatalf("preview %+v %v", preview, e)
	}
	input.PreviewHash = "changed"
	if _, e = s.CreateBackfill(ctx, scope, fid, input, uuid.NewString()); e == nil {
		t.Fatal("unconfirmed preview accepted")
	}
	input.PreviewHash = preview.Hash
	job, e := s.CreateBackfill(ctx, scope, fid, input, uuid.NewString())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Exec(context.Background(), `DELETE FROM flow_operation_job WHERE id=$1`, job.ID)
	defer db.Exec(context.Background(), `DELETE FROM flow_operation_item WHERE job_id=$1`, job.ID)
	for i := 0; i < 2; i++ {
		if e = s.runOperationItem(ctx); e != nil {
			t.Fatal(e)
		}
	}
	job, e = s.OperationJob(ctx, scope, job.ID)
	if e != nil || job.State != "PARTIAL_FAILURE" || job.Items[0].State != "BLOCKED" || job.Items[1].State != "BLOCKED" {
		t.Fatalf("missing approval was not blocked: %+v %v", job, e)
	}
	first, e := s.OperationItemRequest(ctx, scope, job.ID, 0)
	if e != nil {
		t.Fatal(e)
	}
	second, e := s.OperationItemRequest(ctx, scope, job.ID, 1)
	if e != nil || first.ApprovalID != "" || second.ApprovalID != "" || ApprovalDigest(first) == ApprovalDigest(second) {
		t.Fatal("per-occurrence approval scope missing", e)
	}
	if writes.Load() != 0 {
		t.Fatal("engine received an execution before independent approval")
	}
}
