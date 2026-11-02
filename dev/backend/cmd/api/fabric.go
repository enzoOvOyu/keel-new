package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"keel.local/dev/internal/cmdb"
	"keel.local/dev/internal/engineering"
	"net/http"
	"strings"
	"time"
)

type fabricPublishedInput struct {
	ID            string                    `json:"id"`
	Project       string                    `json:"projectId"`
	Revision      int                       `json:"revision"`
	PublishedAt   time.Time                 `json:"publishedAt"`
	ModelRevision string                    `json:"modelRevisionId"`
	InputVersions map[string]int            `json:"inputVersions"`
	Assets        []cmdb.VisualAsset        `json:"assets"`
	AssetVersions map[string]int64          `json:"assetVersions"`
	NetworkPlans  []engineering.NetworkPlan `json:"networkPlans"`
	Digest        string                    `json:"digest"`
	MissingIDs    []string                  `json:"missingAssetIds"`
}
type fabricInputPage struct {
	Project    string                 `json:"projectId"`
	Status     string                 `json:"status"`
	Message    string                 `json:"message"`
	Inputs     []fabricPublishedInput `json:"inputs"`
	NextCursor *string                `json:"nextCursor"`
}

// Fabric consumes immutable Kickoff publications and their CMDB receipts. It
// never creates a Kickoff workspace, re-imports planning data, or writes assets.
func (s *Server) fabricRoutes(w http.ResponseWriter, r *http.Request) bool {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 6 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "projects" || parts[4] != "fabric" || parts[5] != "inputs" {
		return false
	}
	if r.Method != "GET" {
		fail(w, 405, "METHOD_NOT_ALLOWED")
		return true
	}
	project := parts[3]
	actor := subjectOf(r.Context())
	if !actor.Can("fabric:read", project, "", "") || !actor.Can("cmdb:read", project, "", "") {
		fail(w, 403, "FORBIDDEN")
		return true
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	page, err := s.fabricPublishedInputs(ctx, project, r.URL.Query().Get("before"))
	if err != nil {
		fail(w, 500, "FABRIC_INPUT_READ_FAILED")
	} else {
		respond(w, 200, page)
	}
	return true
}
func (s *Server) fabricPublishedInputs(ctx context.Context, project, before string) (fabricInputPage, error) {
	page := fabricInputPage{Project: project, Status: "EMPTY", Message: "尚无已发布且完成CMDB回执的智能开局版本", Inputs: []fabricPublishedInput{}}
	var available bool
	err := s.db.QueryRow(ctx, `SELECT to_regclass('public.kickoff_publication') IS NOT NULL AND to_regclass('public.kickoff_publication_object') IS NOT NULL AND to_regclass('public.kickoff_publication_input') IS NOT NULL`).Scan(&available)
	if err != nil {
		return page, err
	}
	if !available {
		page.Status = "WAITING_FOR_KICKOFF"
		page.Message = "智能开局发布依赖尚未部署；不以演示数据代替已发布CMDB事实"
		return page, nil
	}
	rows, err := s.db.Query(ctx, `SELECT id::text,revision,created_at,model_revision::text FROM kickoff_publication WHERE project_id=$1 AND state='published' AND ($2='' OR revision<(SELECT revision FROM kickoff_publication WHERE id::text=$2 AND project_id=$1)) ORDER BY revision DESC LIMIT 21`, project, before)
	if err != nil {
		return page, err
	}
	for rows.Next() {
		var p fabricPublishedInput
		p.Project = project
		p.Assets = []cmdb.VisualAsset{}
		p.MissingIDs = []string{}
		p.AssetVersions = map[string]int64{}
		p.InputVersions = map[string]int{}
		p.NetworkPlans = []engineering.NetworkPlan{}
		if err = rows.Scan(&p.ID, &p.Revision, &p.PublishedAt, &p.ModelRevision); err != nil {
			rows.Close()
			return page, err
		}
		page.Inputs = append(page.Inputs, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, err
	}
	if len(page.Inputs) > 20 {
		cursor := page.Inputs[19].ID
		page.NextCursor = &cursor
		page.Inputs = page.Inputs[:20]
	}
	for i := range page.Inputs {
		p := &page.Inputs[i]
		ar, e := s.db.Query(ctx, `SELECT o.entity_id::text,c.asset_type,c.name,c.version,c.source_type,c.payload FROM kickoff_publication_object o LEFT JOIN cmdb_entity c ON c.item_uid=o.entity_id::text AND EXISTS(SELECT 1 FROM cmdb_asset_scope sc WHERE sc.asset_id=c.item_uid AND sc.project_id=$2) WHERE o.publication_id::text=$1 ORDER BY o.entity_id`, p.ID, project)
		if e != nil {
			return page, e
		}
		for ar.Next() {
			var a cmdb.VisualAsset
			var kind, name, source *string
			var version *int64
			var raw []byte
			if e = ar.Scan(&a.ID, &kind, &name, &version, &source, &raw); e != nil {
				ar.Close()
				return page, e
			}
			if version == nil {
				p.MissingIDs = append(p.MissingIDs, a.ID)
				continue
			}
			a.Project = project
			a.Type = *kind
			a.Name = *name
			a.Revision = *version
			if source != nil {
				a.Source = *source
			}
			a.Fields = map[string]any{}
			if e = json.Unmarshal(raw, &a.Fields); e != nil {
				ar.Close()
				return page, e
			}
			if fields, ok := a.Fields["fields"].(map[string]any); ok {
				for key, value := range fields {
					a.Fields[key] = value
				}
			}
			p.Assets = append(p.Assets, a)
			p.AssetVersions[a.ID] = a.Revision
		}
		e = ar.Err()
		ar.Close()
		if e != nil {
			return page, e
		}
		ir, e := s.db.Query(ctx, `SELECT table_id,version FROM kickoff_publication_input WHERE publication_id::text=$1 ORDER BY table_id`, p.ID)
		if e != nil {
			return page, e
		}
		for ir.Next() {
			var k string
			var v int
			if e = ir.Scan(&k, &v); e != nil {
				ir.Close()
				return page, e
			}
			p.InputVersions[k] = v
		}
		e = ir.Err()
		ir.Close()
		if e != nil {
			return page, e
		}
		model, _, e := engineeringModel(ctx, s.db, p.ModelRevision, project)
		if e != nil {
			return page, e
		}
		p.NetworkPlans = model.NetworkPlans
		if p.NetworkPlans == nil {
			p.NetworkPlans = []engineering.NetworkPlan{}
		}
		p.Digest = fabricInputDigest(*p)
	}
	if len(page.Inputs) > 0 {
		page.Status = "AVAILABLE"
		page.Message = "只读已发布范围；规划配置与实际观测保持分离"
	}
	return page, nil
}
func fabricInputDigest(p fabricPublishedInput) string {
	value, _ := json.Marshal(struct {
		ID, Model string
		Versions  map[string]int
		Assets    map[string]int64
	}{p.ID, p.ModelRevision, p.InputVersions, p.AssetVersions})
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
