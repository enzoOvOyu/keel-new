package cmdb

import (
	"context"
	"github.com/jackc/pgx/v5"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type visualReadTrace struct{ t *testing.T }
type visualReadTraceKey struct{}

func (v visualReadTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	stage := "other"
	for _, x := range []string{"SELECT md5", "SELECT DISTINCT ON(asset_id", "SELECT e.item_uid,e.asset_type,e.name", "SELECT r.id::text", "walk(id,depth", "con AS MATERIALIZED"} {
		if strings.Contains(d.SQL, x) {
			stage = x
			break
		}
	}
	return context.WithValue(ctx, visualReadTraceKey{}, struct {
		start time.Time
		stage string
	}{time.Now(), stage})
}
func (v visualReadTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	x := ctx.Value(visualReadTraceKey{}).(struct {
		start time.Time
		stage string
	})
	v.t.Logf("query_stage=%s elapsed_ms=%.3f failed=%t", x.stage, float64(time.Since(x.start).Microseconds())/1000, d.Err != nil)
}

// Opt-in only: queries existing data, forces all pooled connections read-only,
// and never enables the integration cases that create synthetic assets.
func TestVisualExistingProjectReadOnly(t *testing.T) {
	if os.Getenv("KEEL_VISUAL_READONLY") != "1" {
		t.Skip("explicit read-only performance run required")
	}
	project := os.Getenv("KEEL_VISUAL_PROJECT")
	if project == "" {
		t.Fatal("KEEL_VISUAL_PROJECT required")
	}
	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal("invalid private database configuration")
	}
	cfg.ConnConfig.Tracer = visualReadTrace{t}
	cfg.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "5000"
	db, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal("database connection failed")
	}
	defer db.Close()
	t.Run("relation_precedence", func(t *testing.T) {
		query := `WITH rel(from_uid,to_uid,fact_layer) AS (VALUES
   ('preferred','old','observed'),('preferred','new','planned'),
   ('ambiguous','a','planned'),('ambiguous','b','planned'),
   ('duplicate','a','planned'),('duplicate','a','planned'),
   ('mixed','a','observed'),('mixed','a','intended'),
   ('unknown','a','observed'),('unknown','b','intended')),
  ranked AS (SELECT *,dense_rank() OVER(PARTITION BY from_uid ORDER BY CASE WHEN fact_layer='planned' THEN 0 ELSE 1 END) preference FROM rel),
  old AS (SELECT from_uid,CASE WHEN count(DISTINCT to_uid)=1 THEN min(to_uid) END target,CASE WHEN count(DISTINCT fact_layer)=1 THEN min(fact_layer) ELSE 'mixed' END layer FROM ranked WHERE preference=1 GROUP BY from_uid),
  revised AS (SELECT from_uid,` + visualPreferredTarget() + ` target,` + visualPreferredLayer() + ` layer FROM rel GROUP BY from_uid)
  SELECT count(*) FROM ((SELECT * FROM old EXCEPT ALL SELECT * FROM revised) UNION ALL (SELECT * FROM revised EXCEPT ALL SELECT * FROM old)) delta`
		var delta int
		if err := db.QueryRow(context.Background(), query).Scan(&delta); err != nil {
			t.Fatal(err)
		}
		if delta != 0 {
			t.Fatal("planned precedence or ambiguity semantics changed")
		}
	})

	s := Service{DB: db}
	scope := Scope{Projects: []string{project}, UserID: "readonly-performance"}
	t.Run("denied_context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var seed string
		if err := db.QueryRow(ctx, `SELECT asset_id FROM cmdb_asset_scope WHERE project_id=$1 ORDER BY asset_id LIMIT 1`, project).Scan(&seed); err != nil {
			t.Fatal(err)
		}
		out, err := s.visualContext(ctx, Scope{}, []string{seed}, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Assets) != 0 || len(out.Relationships) != 0 {
			t.Fatal("context crossed authorization scope")
		}
	})

	t.Run("physical_root", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var root string
		err := db.QueryRow(ctx, `SELECT e.item_uid FROM cmdb_entity e WHERE e.asset_type='server' AND `+activeVisualAsset("e")+`
   AND (SELECT count(*) FROM cmdb_relationship r JOIN cmdb_entity p ON p.item_uid=r.to_uid WHERE r.from_uid=e.item_uid AND r.definition_id='contains' AND r.valid_to IS NULL AND p.asset_type='port')>1
   AND EXISTS(SELECT 1 FROM cmdb_relationship r JOIN cmdb_relationship ep ON ep.to_uid=r.to_uid AND ep.definition_id IN('cable_endpoint_a','cable_endpoint_b') AND ep.valid_to IS NULL WHERE r.from_uid=e.item_uid AND r.definition_id='contains' AND r.valid_to IS NULL)
   ORDER BY e.item_uid LIMIT 1`, scope.Args()...).Scan(&root)
		if err != nil {
			t.Fatal(err)
		}
		expected := map[string]bool{}
		rows, err := db.Query(ctx, `WITH RECURSIVE members(id,depth) AS (
   SELECT $3::text,0 UNION SELECT r.to_uid,m.depth+1 FROM members m JOIN cmdb_relationship r ON r.from_uid=m.id JOIN cmdb_entity child ON child.item_uid=r.to_uid
   WHERE m.depth<8 AND r.definition_id='contains' AND r.valid_to IS NULL AND `+RelationshipVisible("r")+` AND `+activeVisualAsset("child")+`)
   SELECT DISTINCT e.item_uid FROM members m JOIN cmdb_entity e ON e.item_uid=m.id WHERE e.asset_type='port'`, append(scope.Args(), root)...)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			expected[id] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(expected) < 2 {
			t.Fatal("fixture needs multiple root ports")
		}
		started := time.Now()
		out, err := s.VisualSubgraph(ctx, scope, url.Values{"view": {"physical"}, "depth": {"2"}, "root_id": {root}, "limit": {"1"}})
		if err != nil {
			t.Fatal(err)
		}
		if out.Truncated || len(out.Cables) != 1 {
			t.Fatal("focused physical page lost bounded cable context")
		}
		loaded := map[string]bool{}
		endpoints := map[string]bool{}
		for _, a := range out.Assets {
			loaded[a.ID] = true
		}
		for _, c := range out.Cables {
			endpoints[c.A] = true
			endpoints[c.B] = true
			if !loaded[c.A] || !loaded[c.B] || !loaded[c.DeviceA] || !loaded[c.DeviceB] {
				t.Fatal("lost cable endpoint/device")
			}
		}
		rootPorts := 0
		for _, a := range out.Assets {
			if a.Type == "port" {
				if expected[a.ID] {
					rootPorts++
				} else if !endpoints[a.ID] {
					t.Fatal("expanded remote sibling port")
				}
			}
		}
		for id := range expected {
			if !loaded[id] {
				t.Fatal("focused device port missing")
			}
		}
		if rootPorts != len(expected) {
			t.Fatal("root port count differs from contains")
		}
		if !out.Page.HasMore || out.Page.NextCursor == nil {
			t.Fatal("limit=1 must retain continuation")
		}
		t.Logf("elapsed_ms=%.3f root_ports=%d db_contains_ports=%d assets=%d relations=%d cables=%d partial=%t", float64(time.Since(started).Microseconds())/1000, rootPorts, len(expected), len(out.Assets), len(out.Relationships), len(out.Cables), out.Truncated)
	})

	q := url.Values{"limit": {"100"}, "view": {"hierarchy"}, "depth": {"1"}, "axes": {"server_switch"}}
	for _, name := range []string{"subgraph", "matrix", "connections", "physical"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			started := time.Now()
			switch name {
			case "subgraph":
				out, e := s.VisualSubgraph(ctx, scope, q)
				if e != nil {
					t.Fatal(e)
				}
				t.Logf("assets=%d groups=%d more=%t partial=%t", len(out.Assets), len(out.Groups), out.Page.HasMore, out.Truncated)
			case "connections", "physical":
				var out VisualSubgraph
				var e error
				if name == "physical" {
					physicalQ := url.Values{"limit": {"100"}, "view": {"physical"}, "depth": {"2"}}
					out, e = s.VisualSubgraph(ctx, scope, physicalQ)
				} else {
					out, e = s.VisualConnections(ctx, scope, q)
				}
				if e != nil {
					t.Fatal(e)
				}
				t.Logf("assets=%d relations=%d more=%t partial=%t", len(out.Assets), len(out.Relationships), out.Page.HasMore, out.Truncated)
				loaded := map[string]bool{}
				endpoints := map[string]bool{}
				for _, a := range out.Assets {
					loaded[a.ID] = true
				}
				for _, r := range out.Relationships {
					if r.Type == "cable_endpoint_a" || r.Type == "cable_endpoint_b" {
						endpoints[r.To] = true
						if !loaded[r.To] && !out.Truncated {
							t.Fatal("complete context lost cable endpoint")
						}
					}
				}
				for _, a := range out.Assets {
					if a.Type == "port" && !endpoints[a.ID] {
						t.Fatal("connection context expanded sibling ports")
					}
				}
				if name == "physical" {
					if out.Truncated || len(out.Cables) != 100 {
						t.Fatal("physical MM page must retain all 100 cables and their context")
					}
					for _, c := range out.Cables {
						if !loaded[c.A] || !loaded[c.B] || !loaded[c.DeviceA] || !loaded[c.DeviceB] {
							t.Fatal("physical cable lost endpoint or device context")
						}
					}
				}
				if out.Page.HasMore && out.Page.NextCursor == nil {
					t.Fatal("missing continuation cursor")
				}

			case "matrix":
				out, e := s.VisualMatrix(ctx, scope, q)
				if e != nil {
					t.Fatal(e)
				}
				t.Logf("cells=%d included=%d excluded=%d more=%t", len(out.Cells), out.Included, out.Excluded, out.Page.HasMore)
			}
			t.Logf("elapsed_ms=%.3f", float64(time.Since(started).Microseconds())/1000)
		})
	}
}
