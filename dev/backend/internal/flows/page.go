package flows

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type QueryPage struct {
	Limit   int     `json:"limit"`
	HasMore bool    `json:"has_more"`
	Next    *string `json:"next_cursor"`
}
type ExecutionPage struct {
	Items       []Execution `json:"items"`
	Page        QueryPage   `json:"page"`
	StateSource string      `json:"state_filter_source"`
}
type executionCursor struct {
	At    time.Time `json:"at"`
	ID    string    `json:"id"`
	Scope string    `json:"scope"`
}

func (s *Service) ExecutionPage(ctx context.Context, p Scope, q url.Values) (ExecutionPage, error) {
	out := ExecutionPage{Items: []Execution{}, Page: QueryPage{Limit: 50}, StateSource: "Kestra projection; rows refresh from engine"}
	if q.Get("limit") != "" {
		n, e := strconv.Atoi(q.Get("limit"))
		if e != nil || n < 1 || n > 100 {
			return out, problem(400, "FLOWS_LIMIT_INVALID")
		}
		out.Page.Limit = n
	}
	queryCopy := url.Values{}
	for k, v := range q {
		if k != "cursor" {
			queryCopy[k] = v
		}
	}
	projects := append([]string{}, p.Projects...)
	sort.Strings(projects)
	digest := hash(p.User + strings.Join(projects, ",") + queryCopy.Encode())
	cursor := executionCursor{At: time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC), ID: "ffffffff-ffff-ffff-ffff-ffffffffffff"}
	if encoded := q.Get("cursor"); encoded != "" {
		raw, e := base64.RawURLEncoding.DecodeString(encoded)
		if e != nil || json.Unmarshal(raw, &cursor) != nil || cursor.ID == "" || cursor.Scope != digest {
			return out, problem(400, "FLOWS_CURSOR_SCOPE_INVALID")
		}
	}
	rows, e := s.DB.Query(ctx, `SELECT b.id::text,b.created_at FROM flow_execution_binding b WHERE b.project_id=ANY($1) AND (b.created_at,b.id)<($2,$3::uuid) AND ($4='' OR b.id::text ILIKE '%'||$4||'%' OR b.engine_flow_id ILIKE '%'||$4||'%' OR EXISTS(SELECT 1 FROM flow_execution_target t WHERE t.execution_id=b.id AND (t.asset_id ILIKE '%'||$4||'%' OR t.snapshot->>'name' ILIKE '%'||$4||'%'))) AND ($5='' OR b.engine_state=$5) AND ($6='' OR b.source_module=$6) AND ($7='' OR b.flow_id::text=$7) AND ($8='' OR b.requester_id::text=$8) AND ($9='' OR EXISTS(SELECT 1 FROM flow_execution_target t WHERE t.execution_id=b.id AND t.actual_transport=$9)) ORDER BY b.created_at DESC,b.id DESC LIMIT $10`, p.Projects, cursor.At, cursor.ID, q.Get("q"), q.Get("state"), q.Get("source"), q.Get("flow_id"), q.Get("started_by"), q.Get("transport"), out.Page.Limit+1)
	if e != nil {
		return out, e
	}
	type row struct {
		id string
		at time.Time
	}
	ids := []row{}
	for rows.Next() {
		var item row
		if e = rows.Scan(&item.id, &item.at); e != nil {
			rows.Close()
			return out, e
		}
		ids = append(ids, item)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	if len(ids) > out.Page.Limit {
		out.Page.HasMore = true
		ids = ids[:out.Page.Limit]
	}
	for _, item := range ids {
		execution, e := s.Execution(ctx, p, item.id)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, execution)
	}
	if out.Page.HasMore && len(ids) > 0 {
		last := ids[len(ids)-1]
		next := base64.RawURLEncoding.EncodeToString(raw(executionCursor{At: last.at, ID: last.id, Scope: digest}))
		out.Page.Next = &next
	}
	return out, nil
}
