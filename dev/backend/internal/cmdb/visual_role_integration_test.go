package cmdb

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

func TestBOMNetworkRoleFeedsTopologyMatrix(t *testing.T) {
	if os.Getenv("KEEL_CMDB_INTEGRATION") != "1" {
		t.Skip("explicit local PostgreSQL integration run required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	project := "test-role-" + uuid.NewString()
	if _, err = tx.Exec(ctx, "INSERT INTO project(id,name,code) VALUES($1,'独立测试-事务回滚-拓扑角色',$1)", project); err != nil {
		t.Fatal(err)
	}
	for id, kind := range map[string]string{"leaf": "switch", "spine": "switch", "a": "port", "b": "port", "wire": "cable"} {
		payload := "{}"
		if id == "leaf" {
			payload = `{"network_role":"leaf"}`
		}
		if id == "spine" {
			payload = `{"network_role":"spine"}`
		}
		if _, err = tx.Exec(ctx, "INSERT INTO cmdb_entity(item_uid,asset_type,name,payload,source_ref) VALUES($1,$2,$2,$3,'synthetic-role-test')", project+id, kind, []byte(payload)); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO cmdb_asset_scope(project_id,asset_id,source_ref) VALUES($1,$2,'synthetic-role-test')", project, project+id); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range [][3]string{{"leaf", "a", "contains"}, {"spine", "b", "contains"}, {"wire", "a", "cable_endpoint_a"}, {"wire", "b", "cable_endpoint_b"}} {
		if _, err = tx.Exec(ctx, "INSERT INTO cmdb_relationship(from_uid,to_uid,kind,definition_id,project_id,source_ref) VALUES($1,$2,$3,$3,$4,'synthetic-role-test')", project+r[0], project+r[1], r[2], project); err != nil {
			t.Fatal(err)
		}
	}
	expression, err := matrixExpressions("leaf_spine")
	if err != nil {
		t.Fatal(err)
	}
	query := visualCTE() + "SELECT count(*) FROM con c WHERE " + expression[0] + " IS NOT NULL AND " + expression[1] + " IS NOT NULL"
	count := func(scope Scope) int {
		t.Helper()
		var n int
		if err := tx.QueryRow(ctx, query, append(scope.Args(), "")...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if count(Scope{Projects: []string{project}}) != 1 {
		t.Fatal("BOM network_role connection excluded from Leaf-Spine")
	}
	if count(Scope{Projects: []string{"unrelated"}}) != 0 {
		t.Fatal("role fallback crossed project authorization")
	}
	if _, err = tx.Exec(ctx, `UPDATE cmdb_entity SET payload='{"role":"border","network_role":"leaf"}' WHERE item_uid=$1`, project+"leaf"); err != nil {
		t.Fatal(err)
	}
	if count(Scope{Projects: []string{project}}) != 0 {
		t.Fatal("explicit role was overwritten by fallback")
	}
}
