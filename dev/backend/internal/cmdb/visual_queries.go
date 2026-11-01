package cmdb

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type VisualPage struct {
	Limit      int     `json:"limit"`
	HasMore    bool    `json:"has_more"`
	NextCursor *string `json:"next_cursor"`
}
type VisualGroupCounts struct {
	Device int `json:"device"`
	Rack   int `json:"rack"`
	Server int `json:"server"`
	Switch int `json:"switch"`
	GPU    int `json:"gpu"`
	Cable  int `json:"cable"`
}
type VisualGroup struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Counts VisualGroupCounts `json:"counts"`
}
type VisualSubgraph struct {
	Visualization
	Groups []VisualGroup `json:"groups"`
	Page   VisualPage    `json:"page"`
}
type VisualCellGroup struct {
	Fact      string `json:"fact_layer"`
	Media     string `json:"media_type"`
	Speed     string `json:"speed"`
	Count     int    `json:"count"`
	MemberRef string `json:"member_query_ref"`
}
type VisualCell struct {
	Groups    []VisualCellGroup `json:"groups"`
	Row       string            `json:"row_id"`
	Column    string            `json:"column_id"`
	Count     int               `json:"count"`
	MemberRef string            `json:"member_query_ref"`
}
type VisualMatrixCoverage struct {
	Returned int  `json:"returned_cells"`
	Total    int  `json:"total_cells"`
	Limit    int  `json:"limit"`
	Complete bool `json:"complete"`
}
type VisualMatrix struct {
	Snapshot string               `json:"snapshot_id"`
	Rows     []VisualAsset        `json:"rows"`
	Columns  []VisualAsset        `json:"columns"`
	Cells    []VisualCell         `json:"cells"`
	Included int                  `json:"included_count"`
	Excluded int                  `json:"excluded_count"`
	Coverage VisualMatrixCoverage `json:"coverage"`
	Page     VisualPage           `json:"page"`
}
type VisualTraceInput struct {
	Project    string `json:"project_id"`
	Mode       string `json:"mode"`
	From       string `json:"from_port_id"`
	To         string `json:"to_port_id"`
	Snapshot   string `json:"snapshot_id"`
	Candidate  string `json:"candidate_cable_id,omitempty"`
	At         string `json:"at,omitempty"`
	EvidenceAt string `json:"evidence_at,omitempty"`
}
type VisualBreakpoint struct {
	AssetID string `json:"asset_id"`
	Code    string `json:"code"`
	Reason  string `json:"reason"`
}
type VisualTrace struct {
	Snapshot      string             `json:"snapshot_id"`
	Assets        []VisualAsset      `json:"assets"`
	Relationships []VisualRelation   `json:"relationships"`
	Findings      []VisualFinding    `json:"findings"`
	Segments      []VisualCable      `json:"segments"`
	Breakpoints   []VisualBreakpoint `json:"breakpoints"`
	Integrity     string             `json:"integrity"`
	Verification  string             `json:"verification"`
	Coverage      VisualCoverage     `json:"coverage"`
	Reason        string             `json:"reason"`
}
type visualToken struct {
	Kind       string `json:"k"`
	Query      string `json:"q"`
	Revision   string `json:"r"`
	After      string `json:"a,omitempty"`
	Axes       string `json:"x,omitempty"`
	Row        string `json:"u,omitempty"`
	Column     string `json:"v,omitempty"`
	Root       string `json:"o,omitempty"`
	Fabric     string `json:"f,omitempty"`
	Search     string `json:"s,omitempty"`
	Fact       string `json:"l,omitempty"`
	Media      string `json:"m,omitempty"`
	Speed      string `json:"z,omitempty"`
	Classified bool   `json:"c,omitempty"`
	Unplaced   string `json:"p,omitempty"`
}

// Ephemeral cursor authentication key: a process restart invalidates cursors,
// never grants access or retains a historical topology snapshot.
var visualTokenKey = func() []byte {
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		panic(e)
	}
	return key
}()

func signVisualToken(t visualToken) string {
	b, _ := json.Marshal(t)
	m := hmac.New(sha256.New, visualTokenKey)
	m.Write(b)
	return base64.RawURLEncoding.EncodeToString(b) + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
func readVisualToken(raw string) (visualToken, error) {
	var t visualToken
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return t, Reject("VISUAL_CURSOR_INVALID", 400)
	}
	b, e := base64.RawURLEncoding.DecodeString(parts[0])
	if e != nil {
		return t, Reject("VISUAL_CURSOR_INVALID", 400)
	}
	sig, e := base64.RawURLEncoding.DecodeString(parts[1])
	m := hmac.New(sha256.New, visualTokenKey)
	m.Write(b)
	if e != nil || !hmac.Equal(sig, m.Sum(nil)) {
		return t, Reject("VISUAL_CURSOR_INVALID", 400)
	}
	if json.Unmarshal(b, &t) != nil {
		return t, Reject("VISUAL_CURSOR_INVALID", 400)
	}
	return t, nil
}
func visualScopeKey(scope Scope) string {
	p := append([]string{}, scope.Projects...)
	r := append([]string{}, scope.Pairs...)
	sort.Strings(p)
	sort.Strings(r)
	b, _ := json.Marshal([]any{scope.UserID, scope.PermissionVersion, p, r})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func visualQueryKey(scope Scope, path string, q url.Values) string {
	copy := url.Values{}
	for k, vs := range q {
		if k != "cursor" && k != "snapshot_id" {
			copy[k] = append([]string{}, vs...)
		}
	}
	h := sha256.Sum256([]byte(visualScopeKey(scope) + path + copy.Encode()))
	return hex.EncodeToString(h[:])
}
func visualLimit(q url.Values) (int, error) {
	n := 100
	if q.Get("limit") != "" {
		v, e := strconv.Atoi(q.Get("limit"))
		if e != nil {
			return 0, Reject("VISUAL_LIMIT_INVALID", 400)
		}
		n = v
	}
	if n < 1 || n > 200 {
		return 0, Reject("VISUAL_LIMIT_INVALID", 400)
	}
	return n, nil
}
func (s Service) visualRevision(ctx context.Context, scope Scope) (string, error) {
	var revision string
	e := s.DB.QueryRow(ctx, `SELECT md5(COALESCE((SELECT string_agg(e.item_uid||':'||e.version::text,',' ORDER BY e.item_uid) FROM cmdb_entity e WHERE `+activeVisualAsset("e")+`),'')||'|'||COALESCE((SELECT string_agg(r.id::text||':'||r.version::text||':'||COALESCE(r.valid_to::text,''),',' ORDER BY r.id) FROM cmdb_relationship r WHERE `+RelationshipVisible("r")+`),''))`, scope.Args()...).Scan(&revision)
	if e != nil {
		return "", e
	}
	h := sha256.Sum256([]byte(visualScopeKey(scope) + revision))
	return hex.EncodeToString(h[:])[:24], nil
}
func (s Service) visualBegin(ctx context.Context, scope Scope, path string, q url.Values) (string, string, int, error) {
	if q.Get("at") != "" || q.Get("evidence_at") != "" {
		return "", "", 0, Reject("CAPABILITY_UNAVAILABLE", 422)
	}
	if q.Get("sort") != "" && q.Get("sort") != "id" {
		return "", "", 0, Reject("VISUAL_SORT_UNSUPPORTED", 400)
	}

	limit, e := visualLimit(q)
	if e != nil {
		return "", "", 0, e
	}
	revision, e := s.visualRevision(ctx, scope)
	if e != nil {
		return "", "", 0, e
	}
	if q.Get("snapshot_id") != "" && q.Get("snapshot_id") != revision {
		return "", "", 0, Reject("VISUAL_SNAPSHOT_EXPIRED", 409)
	}
	after := ""
	if q.Get("cursor") != "" {
		token, e := readVisualToken(q.Get("cursor"))
		if e != nil {
			return "", "", 0, e
		}
		if token.Kind != "page" || token.Query != visualQueryKey(scope, path, q) {
			return "", "", 0, Reject("VISUAL_CURSOR_SCOPE_MISMATCH", 400)
		}
		if token.Revision != revision {
			return "", "", 0, Reject("VISUAL_SNAPSHOT_EXPIRED", 409)
		}
		after = token.After
	}
	return revision, after, limit, nil
}
func (s Service) visualEnd(ctx context.Context, scope Scope, revision string) error {
	current, e := s.visualRevision(ctx, scope)
	if e != nil {
		return e
	}
	if current != revision {
		return Reject("VISUAL_SNAPSHOT_EXPIRED", 409)
	}
	return nil
}
func visualPage(scope Scope, path string, q url.Values, revision, last string, limit int, more bool) VisualPage {
	p := VisualPage{Limit: limit, HasMore: more}
	if more {
		next := signVisualToken(visualToken{Kind: "page", Query: visualQueryKey(scope, path, q), Revision: revision, After: last})
		p.NextCursor = &next
	}
	return p
}

// Every derived join below is between authorized existing entities and typed
// relations. Geometry, LLDP and display names never create a physical edge.
func visualCTE() string {
	return `WITH RECURSIVE visible AS (SELECT e.* FROM cmdb_entity e WHERE ` + activeVisualAsset("e") + `), rel AS (SELECT r.from_uid,r.to_uid,r.definition_id,r.fact_layer FROM cmdb_relationship r WHERE r.valid_to IS NULL AND ` + RelationshipVisible("r") + ` AND EXISTS(SELECT 1 FROM visible va WHERE va.item_uid=r.from_uid) AND EXISTS(SELECT 1 FROM visible vb WHERE vb.item_uid=r.to_uid)), location_ranked AS (SELECT r.*,dense_rank() OVER(PARTITION BY from_uid ORDER BY CASE WHEN fact_layer='planned' THEN 0 ELSE 1 END) preference FROM rel r WHERE definition_id='located_in'), locations AS (SELECT from_uid,CASE WHEN count(DISTINCT to_uid)=1 THEN min(to_uid) END rack_id FROM location_ranked WHERE preference=1 GROUP BY from_uid), hierarchy(parent_id,child_id) AS (SELECT from_uid,to_uid FROM rel WHERE definition_id IN('parent','contains') UNION SELECT rack_id,from_uid FROM locations WHERE rack_id IS NOT NULL), scope_nodes(id) AS (SELECT item_uid FROM visible WHERE item_uid=$3 UNION SELECT h.child_id FROM hierarchy h JOIN scope_nodes n ON n.id=h.parent_id), owners(port_id,id,depth,path) AS (SELECT item_uid,item_uid,0,ARRAY[item_uid] FROM visible WHERE asset_type='port' UNION ALL SELECT w.port_id,r.from_uid,w.depth+1,w.path||r.from_uid FROM owners w JOIN rel r ON r.to_uid=w.id AND r.definition_id='contains' WHERE w.depth<8 AND NOT r.from_uid=ANY(w.path)), device_owner_candidates AS (SELECT w.port_id,w.id FROM owners w JOIN visible d ON d.item_uid=w.id AND d.asset_type IN('server','switch','pdu','patch_panel','device')), owner_candidates AS (
 SELECT port_id,id FROM device_owner_candidates
 UNION ALL
 SELECT w.port_id,w.id FROM owners w JOIN visible d ON d.item_uid=w.id
 WHERE w.depth=1 AND d.asset_type IN('gpu','nic','psu','bmc','component','optic','electrical_module','connector')
 AND NOT EXISTS(SELECT 1 FROM device_owner_candidates c WHERE c.port_id=w.port_id)
), owner AS (SELECT w.port_id,min(w.id) id FROM owner_candidates w GROUP BY w.port_id HAVING count(DISTINCT w.id)=1), ranked_ends AS (SELECT r.*,dense_rank() OVER(PARTITION BY from_uid,definition_id ORDER BY CASE WHEN fact_layer='planned' THEN 0 ELSE 1 END) preference FROM rel r WHERE definition_id IN('cable_endpoint_a','cable_endpoint_b')), ends AS (SELECT from_uid,definition_id,CASE WHEN count(DISTINCT to_uid)=1 THEN min(to_uid) END port_id,CASE WHEN count(DISTINCT fact_layer)=1 THEN min(fact_layer) ELSE 'mixed' END fact_layer FROM ranked_ends WHERE preference=1 GROUP BY from_uid,definition_id), con AS MATERIALIZED (
 SELECT c.item_uid id,
 CASE WHEN a.fact_layer=b.fact_layer THEN a.fact_layer ELSE 'mixed' END fact_layer,
 COALESCE(c.payload->>'media_type',c.payload->'fields'->>'media_type',c.payload->>'cable_type','') media_type,
 COALESCE(c.payload->>'speed',c.payload->'fields'->>'speed','') speed,
 a.port_id a,b.port_id b,oa.id device_a,ob.id device_b,da.asset_type type_a,db.asset_type type_b,
 COALESCE(NULLIF(da.payload->>'role',''),NULLIF(da.payload->'fields'->>'role',''),da.payload->>'network_role',da.payload->'fields'->>'network_role','') role_a,
 COALESCE(NULLIF(db.payload->>'role',''),NULLIF(db.payload->'fields'->>'role',''),db.payload->>'network_role',db.payload->'fields'->>'network_role','') role_b,
 CASE WHEN la.from_uid IS NOT NULL THEN la.rack_id ELSE pra.item_uid END rack_a,
 CASE WHEN lb.from_uid IS NOT NULL THEN lb.rack_id ELSE prb.item_uid END rack_b
 FROM visible c
 LEFT JOIN ends a ON a.from_uid=c.item_uid AND a.definition_id='cable_endpoint_a'
 LEFT JOIN ends b ON b.from_uid=c.item_uid AND b.definition_id='cable_endpoint_b'
 LEFT JOIN owner oa ON oa.port_id=a.port_id LEFT JOIN owner ob ON ob.port_id=b.port_id
 LEFT JOIN cmdb_entity da ON da.item_uid=oa.id LEFT JOIN cmdb_entity db ON db.item_uid=ob.id
 LEFT JOIN locations la ON la.from_uid=oa.id LEFT JOIN locations lb ON lb.from_uid=ob.id
 LEFT JOIN cmdb_entity pra ON pra.asset_type='rack' AND pra.item_uid=COALESCE(da.payload->>'rack_id',da.payload->'fields'->>'rack_id') AND ` + activeVisualAsset("pra") + `
 LEFT JOIN cmdb_entity prb ON prb.asset_type='rack' AND prb.item_uid=COALESCE(db.payload->>'rack_id',db.payload->'fields'->>'rack_id') AND ` + activeVisualAsset("prb") + `
 WHERE c.asset_type='cable') `
}

func visualArgs(scope Scope, q url.Values, after string) []any {
	return append(scope.Args(), q.Get("root_id"), q.Get("fabric_id"), q.Get("q"), after)
}
func fabricPredicate(alias string) string {
	predicate := `($4='' OR ` + alias + `.item_uid=$4 OR COALESCE(` + alias + `.payload->>'fabric_id',` + alias + `.payload->'fields'->>'fabric_id')=$4 OR EXISTS(SELECT 1 FROM rel fm WHERE fm.from_uid=` + alias + `.item_uid AND fm.to_uid=$4 AND fm.definition_id IN('logical_membership','parent')) OR EXISTS(SELECT 1 FROM rel lm JOIN visible segment ON segment.item_uid=lm.to_uid WHERE lm.definition_id='logical_membership' AND (lm.from_uid=` + alias + `.item_uid OR lm.from_uid IN(SELECT ow.port_id FROM owner ow WHERE ow.id=` + alias + `.item_uid)) AND (segment.item_uid=$4 OR COALESCE(segment.payload->>'fabric_id',segment.payload->'fields'->>'fabric_id')=$4 OR EXISTS(SELECT 1 FROM rel sf WHERE sf.definition_id IN('parent','logical_membership') AND ((sf.from_uid=segment.item_uid AND sf.to_uid=$4) OR (sf.from_uid=$4 AND sf.to_uid=segment.item_uid))))))`
	unassigned := strings.ReplaceAll(predicate, "$4", "known_fabric.item_uid")
	return `(($4='__unassigned__' AND NOT EXISTS(SELECT 1 FROM visible known_fabric WHERE known_fabric.asset_type='fabric' AND ` + unassigned + `)) OR ($4<>'__unassigned__' AND ` + predicate + `))`
}

func entitySearch(alias string) string {
	fields := []string{"serial", "hostname", "management_ip", "bmc_ip", "ip", "mac", "permanent_mac", "physical_name"}
	terms := []string{`$5=''`, alias + `.item_uid ILIKE '%'||$5||'%'`, alias + `.name ILIKE '%'||$5||'%'`}
	for _, field := range fields {
		terms = append(terms, `COALESCE(`+alias+`.payload->>'`+field+`',`+alias+`.payload->'fields'->>'`+field+`','') ILIKE '%'||$5||'%'`)
	}
	return "(" + strings.Join(terms, " OR ") + ")"
}

func connectionPredicate() string {
	return `($3='' OR c.id IN(SELECT id FROM scope_nodes) OR c.a IN(SELECT id FROM scope_nodes) OR c.b IN(SELECT id FROM scope_nodes) OR c.device_a IN(SELECT id FROM scope_nodes) OR c.device_b IN(SELECT id FROM scope_nodes)) AND ($4='' OR EXISTS(SELECT 1 FROM visible f WHERE f.item_uid IN(c.device_a,c.device_b) AND ` + fabricPredicate("f") + `)) AND ($5='' OR c.id ILIKE '%'||$5||'%' OR EXISTS(SELECT 1 FROM visible d WHERE d.item_uid IN(c.a,c.b,c.device_a,c.device_b) AND ` + entitySearch("d") + `))`
}
func visualSeedSQL(where string, limit int) string {
	return visualCTE() + `SELECT v.item_uid FROM visible v WHERE ` + where + ` AND v.item_uid>$6 ORDER BY v.item_uid LIMIT ` + strconv.Itoa(limit+1)
}
func connectionQueryPredicate(q url.Values) string {
	p := connectionPredicate()
	if q.Get("unplaced") == "true" {
		p += " AND (c.rack_a IS NULL OR c.rack_b IS NULL)"
	}
	return p
}
func (s Service) visualIDs(ctx context.Context, scope Scope, q url.Values, after, where string, limit int) ([]string, bool, error) {
	rows, e := s.DB.Query(ctx, visualSeedSQL(where, limit), visualArgs(scope, q, after)...)
	if e != nil {
		return nil, false, e
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return nil, false, e
		}
		ids = append(ids, id)
	}
	if e = rows.Err(); e != nil {
		return nil, false, e
	}
	more := len(ids) > limit
	if more {
		ids = ids[:limit]
	}
	return ids, more, nil
}

// Only the selected seeds and their structural context reach Go. The bounded
// closure never expands arbitrary sibling devices through a rack.
func visualContextSQL() string {
	return `WITH RECURSIVE visible AS (SELECT e.item_uid,e.asset_type FROM cmdb_entity e WHERE ` + activeVisualAsset("e") + `), rel AS (SELECT r.* FROM cmdb_relationship r JOIN visible a ON a.item_uid=r.from_uid JOIN visible b ON b.item_uid=r.to_uid WHERE r.valid_to IS NULL AND ` + RelationshipVisible("r") + `), links(a,b) AS (SELECT from_uid,to_uid FROM rel WHERE definition_id IN('cable_endpoint_a','cable_endpoint_b','located_in') UNION SELECT to_uid,from_uid FROM rel WHERE definition_id IN('parent','contains') UNION SELECT r.from_uid,r.to_uid FROM rel r JOIN visible a ON a.item_uid=r.from_uid WHERE $4::boolean AND r.definition_id='contains' AND a.asset_type NOT IN('data_center','region','pod','rack','fabric')), walk(id,depth,path) AS (SELECT v.item_uid,0,ARRAY[v.item_uid] FROM visible v WHERE v.item_uid=ANY($3::text[]) UNION ALL SELECT l.b,w.depth+1,w.path||l.b FROM walk w JOIN links l ON l.a=w.id WHERE w.depth<8 AND NOT l.b=ANY(w.path)) SELECT DISTINCT id FROM walk ORDER BY id LIMIT 5001`
}
func (s Service) visualContext(ctx context.Context, scope Scope, seeds []string, rich bool) (Visualization, error) {
	args := append(scope.Args(), seeds, rich)
	rows, e := s.DB.Query(ctx, visualContextSQL(), args...)
	if e != nil {
		return Visualization{}, e
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return Visualization{}, e
		}
		ids = append(ids, id)
	}
	if e = rows.Err(); e != nil {
		return Visualization{}, e
	}
	rows.Close()
	partial := len(ids) > 5000
	// Keep the lookahead ID so the shared reader detects truncation before
	// deriving component counts from an incomplete context.
	out, e := s.visualizationRead(ctx, scope, "", "", true, 5000, ids)
	if partial {
		out.Truncated = true
		out.Coverage.Complete = false
	}
	return out, e
}
func (s Service) VisualSubgraph(ctx context.Context, scope Scope, q url.Values) (VisualSubgraph, error) {
	path := "subgraph"
	rev, after, limit, e := s.visualBegin(ctx, scope, path, q)
	if e != nil {
		return VisualSubgraph{}, e
	}
	view := q.Get("view")
	if view == "" {
		view = "hierarchy"
	}
	if view != "hierarchy" && view != "fabric" && view != "physical" {
		return VisualSubgraph{}, Reject("VISUAL_VIEW_INVALID", 400)
	}
	if d := q.Get("depth"); d != "" && d != "1" && d != "2" {
		return VisualSubgraph{}, Reject("VISUAL_DEPTH_INVALID", 400)
	}
	where := `v.asset_type IN('data_center','region','pod','rack','server','switch','pdu','patch_panel','device') AND ($3='' OR v.item_uid IN(SELECT id FROM scope_nodes)) AND ` + fabricPredicate("v") + ` AND ` + entitySearch("v")

	if q.Get("unplaced") == "true" {
		where += ` AND ((v.asset_type='rack' AND NOT EXISTS(SELECT 1 FROM hierarchy h JOIN visible p ON p.item_uid=h.parent_id AND p.asset_type='pod' WHERE h.child_id=v.item_uid) AND NOT EXISTS(SELECT 1 FROM visible p WHERE p.asset_type='pod' AND p.item_uid=COALESCE(v.payload->>'pod_id',v.payload->'fields'->>'pod_id'))) OR (v.asset_type IN('server','switch','pdu','patch_panel','device') AND NOT EXISTS(SELECT 1 FROM rel l JOIN visible r ON r.item_uid=l.to_uid AND r.asset_type='rack' WHERE l.definition_id='located_in' AND l.from_uid=v.item_uid) AND NOT EXISTS(SELECT 1 FROM visible r WHERE r.asset_type='rack' AND r.item_uid=COALESCE(v.payload->>'rack_id',v.payload->'fields'->>'rack_id'))))`
	}
	if view == "physical" {
		where = `v.item_uid IN(SELECT c.id FROM con c WHERE ` + connectionQueryPredicate(q) + `)`
	}
	ids, more, e := s.visualIDs(ctx, scope, q, after, where, limit)
	if e != nil {
		return VisualSubgraph{}, e
	}
	contextIDs := append([]string{}, ids...)
	if view == "physical" && q.Get("root_id") != "" {
		contextIDs = append(contextIDs, q.Get("root_id"))
	}
	snapshot, e := s.visualContext(ctx, scope, contextIDs, view == "physical" || q.Get("depth") == "2")
	if e != nil {
		return VisualSubgraph{}, e
	}
	snapshot.SnapshotID = rev
	groups, e := s.visualGroups(ctx, scope, q)
	if e != nil {
		return VisualSubgraph{}, e
	}

	if len(groups) > 200 {
		groups = groups[:200]
		snapshot.Truncated = true
		snapshot.Coverage.Complete = false
	}
	groupIndex := map[string]VisualGroup{}
	for _, group := range groups {
		groupIndex[group.ID] = group
	}
	loaded := map[string]int{}
	for _, asset := range snapshot.Assets {
		if asset.Type == "server" || asset.Type == "switch" || asset.Type == "pdu" || asset.Type == "patch_panel" || asset.Type == "device" {
			if id, ok := asset.Fields["rack_id"].(string); ok {
				loaded[id]++
			}
		}
	}
	for i := range snapshot.Assets {
		asset := &snapshot.Assets[i]
		if asset.Type != "rack" {
			continue
		}
		if group, ok := groupIndex[asset.ID]; ok {
			asset.Fields["registered_device_count"] = group.Counts.Device
			asset.Fields["placement_coverage"] = map[string]any{"loaded": loaded[asset.ID], "total": group.Counts.Device, "complete": loaded[asset.ID] == group.Counts.Device && !snapshot.Truncated}
			asset.Fields["visualization_summary"] = loaded[asset.ID] != group.Counts.Device || snapshot.Truncated
		} else {
			asset.Fields["visualization_summary"] = true
		}
	}
	last := ""
	if len(ids) > 0 {
		last = ids[len(ids)-1]
	}
	page := visualPage(scope, path, q, rev, last, limit, more)
	if more {
		snapshot.Coverage.Complete = false
	}
	if e = s.visualEnd(ctx, scope, rev); e != nil {
		return VisualSubgraph{}, e
	}
	return VisualSubgraph{snapshot, groups, page}, nil
}
func visualGroupSQL() string {
	// Count assets and Cable memberships independently. Joining every descendant
	// to every Cable via an OR predicate creates a large intermediate Cartesian
	// relation and can trigger seconds of planning/JIT even for one small project.
	return visualCTE() + `, descendants(root,id) AS (
  SELECT v.item_uid,v.item_uid FROM visible v WHERE v.asset_type IN('data_center','region','pod','rack','fabric')
  UNION SELECT d.root,h.child_id FROM descendants d JOIN hierarchy h ON h.parent_id=d.id
 ), asset_counts AS (
  SELECT d.root,
   count(*) FILTER(WHERE a.asset_type IN('server','switch','pdu','patch_panel','device')) device_count,
   count(*) FILTER(WHERE a.asset_type='rack') rack_count,
   count(*) FILTER(WHERE a.asset_type='server') server_count,
   count(*) FILTER(WHERE a.asset_type='switch') switch_count,
   count(*) FILTER(WHERE a.asset_type='gpu') gpu_count
  FROM descendants d JOIN cmdb_entity a ON a.item_uid=d.id GROUP BY d.root
 ), cable_members AS (
  SELECT d.root,c.id FROM descendants d JOIN con c ON c.device_a=d.id
  UNION SELECT d.root,c.id FROM descendants d JOIN con c ON c.device_b=d.id
 ), cable_counts AS (SELECT root,count(*) n FROM cable_members GROUP BY root)
 SELECT g.item_uid,g.name,g.asset_type,ac.device_count,ac.rack_count,ac.server_count,ac.switch_count,ac.gpu_count,COALESCE(cc.n,0)
 FROM cmdb_entity g JOIN asset_counts ac ON ac.root=g.item_uid LEFT JOIN cable_counts cc ON cc.root=g.item_uid
 WHERE ($3='' OR g.item_uid IN(SELECT id FROM scope_nodes))
 AND ($4='' OR EXISTS(SELECT 1 FROM descendants fd JOIN visible fv ON fv.item_uid=fd.id WHERE fd.root=g.item_uid AND ` + fabricPredicate("fv") + `))
 AND ($5='' OR EXISTS(SELECT 1 FROM descendants ds JOIN visible v ON v.item_uid=ds.id WHERE ds.root=g.item_uid AND ` + entitySearch("v") + `))
 ORDER BY g.item_uid LIMIT 201`
}

func (s Service) visualGroups(ctx context.Context, scope Scope, q url.Values) ([]VisualGroup, error) {
	rows, e := s.DB.Query(ctx, visualGroupSQL(), visualArgs(scope, q, "")[:5]...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	groups := []VisualGroup{}
	for rows.Next() {
		var g VisualGroup
		if e = rows.Scan(&g.ID, &g.Name, &g.Type, &g.Counts.Device, &g.Counts.Rack, &g.Counts.Server, &g.Counts.Switch, &g.Counts.GPU, &g.Counts.Cable); e != nil {
			return nil, e
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}
func (s Service) VisualConnections(ctx context.Context, scope Scope, q url.Values) (VisualSubgraph, error) {

	qCopy := url.Values{}
	for k, vs := range q {
		qCopy[k] = append([]string{}, vs...)
	}
	q = qCopy
	if raw := q.Get("member_query_ref"); raw != "" {
		token, err := readVisualToken(raw)
		if err != nil {
			return VisualSubgraph{}, err
		}
		if token.Kind != "member" || token.Query != visualScopeKey(scope) {
			return VisualSubgraph{}, Reject("VISUAL_MEMBER_SCOPE_MISMATCH", 400)
		}
		for key, value := range map[string]string{"root_id": token.Root, "fabric_id": token.Fabric, "q": token.Search, "unplaced": token.Unplaced} {
			if value != "" {
				if q.Get(key) != "" && q.Get(key) != value {
					return VisualSubgraph{}, Reject("VISUAL_MEMBER_SCOPE_MISMATCH", 400)
				}
				q.Set(key, value)
			}
		}
	}
	path := "connections"
	rev, after, limit, e := s.visualBegin(ctx, scope, path, q)
	if e != nil {
		return VisualSubgraph{}, e
	}
	extra := ""
	if q.Get("unplaced") == "true" {
		extra += " AND (c.rack_a IS NULL OR c.rack_b IS NULL)"
	}
	if member := q.Get("member_query_ref"); member != "" {
		token, err := readVisualToken(member)
		if err != nil {
			return VisualSubgraph{}, err
		}
		if token.Kind != "member" || token.Query != visualScopeKey(scope) {
			return VisualSubgraph{}, Reject("VISUAL_MEMBER_SCOPE_MISMATCH", 400)
		}
		if token.Revision != rev {
			return VisualSubgraph{}, Reject("VISUAL_SNAPSHOT_EXPIRED", 409)
		}
		expr, err := matrixExpressions(token.Axes)
		if err != nil {
			return VisualSubgraph{}, err
		}
		extra = " AND " + expr[0] + "=" + quoteVisual(token.Row) + " AND " + expr[1] + "=" + quoteVisual(token.Column)
		if token.Classified {
			extra += " AND c.fact_layer=" + quoteVisual(token.Fact) + " AND c.media_type=" + quoteVisual(token.Media) + " AND c.speed=" + quoteVisual(token.Speed)
		}
	}
	if id := q.Get("asset_id"); id != "" {
		extra += " AND " + quoteVisual(id) + " IN(c.id,c.device_a,c.device_b)"
	}
	if id := q.Get("port_id"); id != "" {
		extra += " AND " + quoteVisual(id) + " IN(c.a,c.b)"
	}
	if status := q.Get("status"); status != "" {
		if status == "incomplete" {
			extra += " AND (c.a IS NULL OR c.b IS NULL OR c.device_a IS NULL OR c.device_b IS NULL)"
		} else if status != "all" {
			return VisualSubgraph{}, Reject("VISUAL_STATUS_UNSUPPORTED", 400)
		}
	}
	where := `v.item_uid IN(SELECT c.id FROM con c WHERE ` + connectionQueryPredicate(q) + extra + `)`
	ids, more, e := s.visualIDs(ctx, scope, q, after, where, limit)
	if e != nil {
		return VisualSubgraph{}, e
	}

	last := ""
	if len(ids) > 0 {
		last = ids[len(ids)-1]
	}
	for _, key := range []string{"asset_id", "port_id"} {
		if id := q.Get(key); id != "" {
			var valid bool
			args := append(scope.Args(), id)
			condition := ""
			if key == "port_id" {
				condition = " AND e.asset_type='port'"
			}
			if err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cmdb_entity e WHERE `+activeVisualAsset("e")+` AND e.item_uid=$3`+condition+`)`, args...).Scan(&valid); err != nil {
				return VisualSubgraph{}, err
			}
			if !valid {
				return VisualSubgraph{}, Reject("VISUAL_ASSET_NOT_FOUND", 404)
			}
			ids = append(ids, id)
		}
	}
	contextIDs := append([]string{}, ids...)
	for _, key := range []string{"asset_id", "port_id"} {
		if id := q.Get(key); id != "" {
			contextIDs = append(contextIDs, id)
		}
	}
	snapshot, e := s.visualContext(ctx, scope, contextIDs, true)
	if e != nil {
		return VisualSubgraph{}, e
	}
	snapshot.SnapshotID = rev
	for i := range snapshot.Assets {
		if snapshot.Assets[i].Type == "rack" {
			snapshot.Assets[i].Fields["visualization_summary"] = true
		}
	}
	if more {
		snapshot.Coverage.Complete = false
	}
	if e = s.visualEnd(ctx, scope, rev); e != nil {
		return VisualSubgraph{}, e
	}
	return VisualSubgraph{snapshot, []VisualGroup{}, visualPage(scope, path, q, rev, last, limit, more)}, nil
}

// Values originate from decoded scope-bound tokens or query strings, and are
// escaped as SQL literals. They are never accepted as identifiers or SQL text.
func quoteVisual(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func matrixExpressions(axes string) ([2]string, error) {
	switch axes {
	case "server_switch":
		return [2]string{`CASE WHEN c.type_a='server' AND c.type_b='switch' THEN c.device_a WHEN c.type_b='server' AND c.type_a='switch' THEN c.device_b END`, `CASE WHEN c.type_a='server' AND c.type_b='switch' THEN c.device_b WHEN c.type_b='server' AND c.type_a='switch' THEN c.device_a END`}, nil
	case "leaf_spine":
		return [2]string{`CASE WHEN lower(c.role_a) LIKE '%leaf%' AND lower(c.role_b) LIKE '%spine%' THEN c.device_a WHEN lower(c.role_b) LIKE '%leaf%' AND lower(c.role_a) LIKE '%spine%' THEN c.device_b END`, `CASE WHEN lower(c.role_a) LIKE '%leaf%' AND lower(c.role_b) LIKE '%spine%' THEN c.device_b WHEN lower(c.role_b) LIKE '%leaf%' AND lower(c.role_a) LIKE '%spine%' THEN c.device_a END`}, nil
	case "rack_leaf":
		return [2]string{`CASE WHEN c.type_a='server' AND lower(c.role_b) LIKE '%leaf%' THEN c.rack_a WHEN c.type_b='server' AND lower(c.role_a) LIKE '%leaf%' THEN c.rack_b END`, `CASE WHEN c.type_a='server' AND lower(c.role_b) LIKE '%leaf%' THEN c.device_b WHEN c.type_b='server' AND lower(c.role_a) LIKE '%leaf%' THEN c.device_a END`}, nil
	case "port_pair":
		return [2]string{"c.a", "c.b"}, nil
	}
	return [2]string{}, Reject("VISUAL_AXES_INVALID", 400)
}
func (s Service) VisualMatrix(ctx context.Context, scope Scope, q url.Values) (VisualMatrix, error) {
	path := "matrix"
	rev, after, limit, e := s.visualBegin(ctx, scope, path, q)
	if e != nil {
		return VisualMatrix{}, e
	}
	axes := q.Get("axes")
	if axes == "" {
		axes = "server_switch"
	}
	expr, e := matrixExpressions(axes)
	if e != nil {
		return VisualMatrix{}, e
	}
	base := visualCTE() + `, selected AS (SELECT c.*,` + expr[0] + ` row_id,` + expr[1] + ` column_id FROM con c WHERE ` + connectionQueryPredicate(q) + `), classified AS (SELECT row_id,column_id,fact_layer,media_type,speed,count(*) n FROM selected WHERE row_id IS NOT NULL AND column_id IS NOT NULL AND a IS NOT NULL AND b IS NOT NULL AND device_a IS NOT NULL AND device_b IS NOT NULL GROUP BY row_id,column_id,fact_layer,media_type,speed), cells AS (SELECT row_id,column_id,sum(n) n,jsonb_agg(jsonb_build_object('fact_layer',fact_layer,'media_type',media_type,'speed',speed,'count',n) ORDER BY fact_layer,media_type,speed) groups FROM classified GROUP BY row_id,column_id) `
	rows, e := s.DB.Query(ctx, base+`SELECT row_id,column_id,n,groups FROM cells WHERE row_id||chr(31)||column_id>$6 ORDER BY row_id,column_id LIMIT `+strconv.Itoa(limit+1), visualArgs(scope, q, after)...)
	if e != nil {
		return VisualMatrix{}, e
	}
	cells := []VisualCell{}
	for rows.Next() {
		var c VisualCell
		var raw []byte
		if e = rows.Scan(&c.Row, &c.Column, &c.Count, &raw); e != nil {
			rows.Close()
			return VisualMatrix{}, e
		}
		token := visualToken{Kind: "member", Query: visualScopeKey(scope), Revision: rev, Axes: axes, Row: c.Row, Column: c.Column, Root: q.Get("root_id"), Fabric: q.Get("fabric_id"), Search: q.Get("q"), Unplaced: q.Get("unplaced")}
		c.MemberRef = signVisualToken(token)
		if e = json.Unmarshal(raw, &c.Groups); e != nil {
			rows.Close()
			return VisualMatrix{}, e
		}
		for i := range c.Groups {
			g := &c.Groups[i]
			groupToken := token
			groupToken.Classified = true
			groupToken.Fact = g.Fact
			groupToken.Media = g.Media
			groupToken.Speed = g.Speed
			g.MemberRef = signVisualToken(groupToken)
		}
		cells = append(cells, c)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return VisualMatrix{}, e
	}
	more := len(cells) > limit
	if more {
		cells = cells[:limit]
	}
	ids := []string{}
	for _, c := range cells {
		ids = append(ids, c.Row, c.Column)
	}
	snapshot, e := s.visualContext(ctx, scope, ids, false)
	if e != nil {
		return VisualMatrix{}, e
	}
	byID := map[string]VisualAsset{}
	for _, a := range snapshot.Assets {
		byID[a.ID] = a
	}
	out := VisualMatrix{Snapshot: rev, Rows: []VisualAsset{}, Columns: []VisualAsset{}, Cells: cells}
	seenRows, seenCols := map[string]bool{}, map[string]bool{}
	for _, c := range cells {
		if !seenRows[c.Row] {
			out.Rows = append(out.Rows, byID[c.Row])
			seenRows[c.Row] = true
		}
		if !seenCols[c.Column] {
			out.Columns = append(out.Columns, byID[c.Column])
			seenCols[c.Column] = true
		}
	}
	e = s.DB.QueryRow(ctx, base+`SELECT COALESCE((SELECT sum(n) FROM cells),0), (SELECT count(*) FROM selected)-COALESCE((SELECT sum(n) FROM cells),0),(SELECT count(*) FROM cells)`, visualArgs(scope, q, "")[:5]...).Scan(&out.Included, &out.Excluded, &out.Coverage.Total)
	if e != nil {
		return VisualMatrix{}, e
	}
	last := ""
	if len(cells) > 0 {
		c := cells[len(cells)-1]
		last = c.Row + string(rune(31)) + c.Column
	}
	out.Page = visualPage(scope, path, q, rev, last, limit, more)
	out.Coverage.Returned = len(cells)
	out.Coverage.Limit = limit
	out.Coverage.Complete = !more && !snapshot.Truncated
	if e = s.visualEnd(ctx, scope, rev); e != nil {
		return VisualMatrix{}, e
	}
	return out, nil
}
func (s Service) VisualTrace(ctx context.Context, scope Scope, in VisualTraceInput) (VisualTrace, error) {
	if in.At != "" || in.EvidenceAt != "" {
		return VisualTrace{}, Reject("CAPABILITY_UNAVAILABLE", 422)
	}
	if in.Mode == "logical" {
		return VisualTrace{}, Reject("CAPABILITY_UNAVAILABLE", 422)
	}
	if in.Mode != "physical" || in.From == "" {
		return VisualTrace{}, Reject("VISUAL_TRACE_INPUT_INVALID", 400)
	}

	for _, portID := range []string{in.From, in.To} {
		if portID == "" {
			continue
		}
		var valid bool
		args := append(scope.Args(), portID)
		if err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cmdb_entity e WHERE `+activeVisualAsset("e")+` AND e.item_uid=$3 AND e.asset_type='port')`, args...).Scan(&valid); err != nil {
			return VisualTrace{}, err
		}
		if !valid {
			return VisualTrace{}, Reject("VISUAL_PORT_NOT_FOUND", 404)
		}
	}
	q := url.Values{"port_id": {in.From}, "limit": {"200"}}
	if in.Snapshot != "" {
		q.Set("snapshot_id", in.Snapshot)
	}
	page, e := s.VisualConnections(ctx, scope, q)
	if e != nil {
		return VisualTrace{}, e
	}
	out := VisualTrace{Snapshot: page.SnapshotID, Assets: page.Assets, Relationships: page.Relationships, Findings: page.Findings, Segments: []VisualCable{}, Breakpoints: []VisualBreakpoint{}, Integrity: "no_record", Verification: "unverified", Coverage: page.Coverage, Reason: "没有登记的物理线缆；不推断交换机内部转发或未记录跳接"}
	for _, c := range page.Cables {
		if c.A == in.From || c.B == in.From {
			out.Segments = append(out.Segments, c)
		}
	}

	if in.Candidate != "" {
		selected := []VisualCable{}
		for _, c := range out.Segments {
			if c.ID == in.Candidate {
				selected = append(selected, c)
			}
		}
		if len(selected) == 0 {
			return VisualTrace{}, Reject("VISUAL_TRACE_BRANCH_INVALID", 422)
		}
		out.Segments = selected
	}
	if len(out.Segments) > 1 {
		out.Integrity = "ambiguous"
		out.Reason = "起始端口有多条线缆，不能选择一条冒充完整路径"
		out.Breakpoints = append(out.Breakpoints, VisualBreakpoint{in.From, "PORT_OCCUPIED", out.Reason})
	} else if len(out.Segments) == 1 {
		c := out.Segments[0]
		out.Integrity = "complete"
		out.Reason = "仅证明该 Cable 的登记双端；没有验证实际链路可用或交换机转发"
		peer := c.B
		if c.B == in.From {
			peer = c.A
		}
		if peer == "" || c.DeviceA == "" || c.DeviceB == "" {
			out.Integrity = "incomplete"
			out.Reason = "线缆缺少可见且有效的端口/设备端点"
		} else if in.To != "" && peer != in.To {
			out.Integrity = "incomplete"
			out.Reason = "目标不在本条 Cable 对端；缺少有证据的中间贯通段"
		}
		if out.Integrity != "complete" {
			out.Breakpoints = append(out.Breakpoints, VisualBreakpoint{in.From, "TRACE_GAP", out.Reason})
		}
	}
	if !page.Coverage.Complete {
		out.Integrity = "incomplete"
		out.Reason = "查询范围被截断，不能证明路径完整"
	}
	return out, nil
}
