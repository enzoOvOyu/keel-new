package flows

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPersistentBootstrapProgressAndUnknownRecovery(t *testing.T) {
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
	asset := "synthetic-bootstrap-" + uuid.NewString()
	trust, agent := uuid.NewString(), uuid.NewString()
	if _, e = db.Exec(ctx, `INSERT INTO cmdb_entity(item_uid,payload,asset_type,name,source_type,source_ref,maintenance_state,lifecycle_state) VALUES($1,'{"management_ip":"192.0.2.1","os":"Linux"}','server','Synthetic bootstrap','manual','synthetic-operation','active','discovered')`, asset); e != nil {
		t.Fatal(e)
	}
	defer db.Exec(context.Background(), `DELETE FROM cmdb_entity WHERE item_uid=$1`, asset)
	if _, e = db.Exec(ctx, `INSERT INTO cmdb_asset_scope(project_id,asset_id,source_ref) VALUES($1,$2,'synthetic-operation')`, project, asset); e != nil {
		t.Fatal(e)
	}
	defer db.Exec(context.Background(), `DELETE FROM cmdb_asset_scope WHERE project_id=$1 AND asset_id=$2`, project, asset)
	if _, e = db.Exec(ctx, `INSERT INTO flow_ssh_trust(id,project_id,asset_id,host,username,secret_ref,known_hosts_ref,approved_by,status) VALUES($1,$2,$3,'192.0.2.1','synthetic','synthetic/key.pem','synthetic/known_hosts',$4,'approved')`, trust, project, asset, user); e != nil {
		t.Fatal(e)
	}
	defer db.Exec(context.Background(), `DELETE FROM flow_ssh_trust WHERE id=$1`, trust)
	s := &Service{DB: db}
	scope := Scope{User: user, Actor: "synthetic-job", Projects: []string{project}}
	input := BootstrapBatch{Project: project, Assets: []string{asset}, Version: "0.2.0"}
	key := uuid.NewString()
	job, e := s.CreateBootstrapBatch(ctx, scope, input, key)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Exec(context.Background(), `DELETE FROM flow_operation_job WHERE id=$1`, job.ID)
	defer db.Exec(context.Background(), `DELETE FROM flow_operation_item WHERE job_id=$1`, job.ID)
	same, e := s.CreateBootstrapBatch(ctx, scope, input, key)
	if e != nil || same.ID != job.ID {
		t.Fatal("idempotent create failed", e)
	}
	paused, e := s.OperationAction(ctx, scope, job.ID, OperationAction{Action: "pause", Revision: job.Revision})
	if e != nil || paused.State != "PAUSED" {
		t.Fatal("pause failed", e)
	}
	if _, e = s.OperationAction(ctx, scope, job.ID, OperationAction{Action: "resume", Revision: job.Revision}); e == nil {
		t.Fatal("stale revision accepted")
	}
	job, e = s.OperationAction(ctx, scope, job.ID, OperationAction{Action: "resume", Revision: paused.Revision})
	if e != nil {
		t.Fatal(e)
	}
	t.Setenv("KEEL_AGENT_PACKAGE_URL", "")
	if e = s.runOperationItem(ctx); e != nil {
		t.Fatal(e)
	}
	job, e = s.OperationJob(ctx, scope, job.ID)
	if e != nil || job.State != "PARTIAL_FAILURE" || job.Items[0].State != "BLOCKED" {
		t.Fatalf("blocked progress %+v %v", job, e)
	}
	if _, e = db.Exec(ctx, `UPDATE flow_operation_item SET state='RUNNING',started_at=now(),lease_until=now()-interval '1 second' WHERE job_id=$1`, job.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, `UPDATE flow_operation_job SET state='RUNNING' WHERE id=$1`, job.ID); e != nil {
		t.Fatal(e)
	}
	if e = s.runOperationItem(ctx); e != nil {
		t.Fatal(e)
	}
	job, e = s.OperationJob(ctx, scope, job.ID)
	if e != nil || job.Items[0].State != "UNKNOWN" {
		t.Fatal("interrupted operation was silently retried", e)
	}
	if _, e = db.Exec(ctx, `INSERT INTO flow_agent(id,project_id,asset_id,identity,version,capabilities,state,last_heartbeat) VALUES($1,$2,$3,'{}','0.2.0','[]','enrolled',now())`, agent, project, asset); e != nil {
		t.Fatal(e)
	}
	defer db.Exec(context.Background(), `DELETE FROM flow_agent WHERE id=$1`, agent)
	job, e = s.OperationAction(ctx, scope, job.ID, OperationAction{Action: "reconcile", Revision: job.Revision})
	if e != nil || job.State != "COMPLETED" || job.Items[0].State != "SUCCEEDED" {
		t.Fatalf("registry reconciliation %+v %v", job, e)
	}
	if _, e = s.OperationJob(ctx, Scope{User: user, Projects: []string{"unauthorized"}}, job.ID); e == nil {
		t.Fatal("cross-project job exposed")
	}
}
