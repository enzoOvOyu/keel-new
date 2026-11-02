package flows

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type RecentExecution struct {
	ID        string    `json:"id"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	Success   int       `json:"success"`
	Failed    int       `json:"failed"`
	Total     int       `json:"total"`
}
type CatalogItem struct {
	Flow
	Recent *RecentExecution `json:"recent_execution"`
}
type CatalogPage struct {
	Items       []CatalogItem `json:"items"`
	Total       int           `json:"total"`
	Limit       int           `json:"limit"`
	Offset      int           `json:"offset"`
	Namespaces  []string      `json:"namespaces"`
	StateSource string        `json:"state_source"`
}

func queryInt(q url.Values, key string, fallback, max int) (int, error) {
	if q.Get(key) == "" {
		return fallback, nil
	}
	n, e := strconv.Atoi(q.Get(key))
	if e != nil || n < 0 || n > max {
		return 0, problem(400, "FLOWS_PAGINATION_INVALID")
	}
	return n, nil
}
func flowStatus(f Flow) string {
	if len(f.Engine) == 0 {
		return "unknown"
	}
	if f.Draft {
		return "DRAFT"
	}
	if f.Disabled {
		return "DISABLED"
	}
	return "ENABLED"
}
func catalogFilter(all []Flow, q url.Values) ([]Flow, []string) {
	out := []Flow{}
	names := map[string]bool{}
	for _, f := range all {
		names[f.Namespace] = true
		if q.Get("namespace") != "" && f.Namespace != q.Get("namespace") {
			continue
		}
		if q.Get("target_type") != "" && f.TargetType != q.Get("target_type") {
			continue
		}
		if q.Get("state") != "" && flowStatus(f) != q.Get("state") {
			continue
		}
		if !strings.Contains(strings.ToLower(f.Name+" "+f.FlowID+" "+f.Namespace+" "+f.Description), strings.ToLower(q.Get("q"))) {
			continue
		}
		out = append(out, f)
	}
	ns := []string{}
	for n := range names {
		ns = append(ns, n)
	}
	sort.Strings(ns)
	return out, ns
}
func (s *Service) Catalog(ctx context.Context, p Scope, q url.Values) (CatalogPage, error) {
	out := CatalogPage{Items: []CatalogItem{}, Namespaces: []string{}, StateSource: "Flow state: Kestra live; recent execution: persisted Kestra projection"}
	var e error
	out.Limit, e = queryInt(q, "limit", 25, 100)
	if e != nil || out.Limit < 1 {
		return out, problem(400, "FLOWS_LIMIT_INVALID")
	}
	out.Offset, e = queryInt(q, "offset", 0, 1000000)
	if e != nil {
		return out, e
	}
	if !contains([]string{"", "name", "revision", "id"}, q.Get("sort")) || !contains([]string{"", "asc", "desc"}, q.Get("direction")) {
		return out, problem(400, "FLOWS_SORT_INVALID")
	}
	// The engine owns enabled/draft state. Filter its authorized metadata server-side,
	// then send only the requested page; do not create another writable state store.
	all, e := s.Flows(ctx, p)
	if e != nil {
		return out, e
	}
	if q.Get("source_search") == "true" {
		if len(strings.TrimSpace(q.Get("q"))) < 2 {
			return out, problem(400, "SOURCE_QUERY_REQUIRED")
		}
		matches := map[string]bool{}
		namespaces := map[string]bool{}
		for _, f := range all {
			namespaces[f.Namespace] = true
		}
		for ns := range namespaces {
			for page := 1; ; page++ {
				var result struct {
					Results []struct {
						Namespace string `json:"namespace"`
						ID        string `json:"id"`
					} `json:"results"`
					Total int `json:"total"`
				}
				params := url.Values{"namespace": {ns}, "q": {q.Get("q")}, "page": {strconv.Itoa(page)}, "size": {"100"}}
				if err := s.Engine.JSON(ctx, "GET", "/flows/source?"+params.Encode(), &result); err != nil {
					return out, err
				}
				for _, hit := range result.Results {
					if hit.Namespace == ns {
						matches[ns+":"+hit.ID] = true
					}
				}
				if page*100 >= result.Total || len(result.Results) == 0 {
					break
				}
			}
		}
		matched := []Flow{}
		for _, f := range all {
			if matches[f.Namespace+":"+f.FlowID] {
				matched = append(matched, f)
			}
		}
		filtered := url.Values{}
		for key, values := range q {
			filtered[key] = values
		}
		filtered.Del("q")
		all, out.Namespaces = catalogFilter(matched, filtered)
	} else {
		all, out.Namespaces = catalogFilter(all, q)
	}
	out.Total = len(all)
	if q.Get("sort") != "" {
		sort.SliceStable(all, func(i, j int) bool {
			a, b := all[i], all[j]
			cmp := strings.Compare(a.Name, b.Name)
			if q.Get("sort") == "revision" {
				cmp = a.Revision - b.Revision
			}
			if q.Get("sort") == "id" {
				cmp = strings.Compare(a.ID, b.ID)
			}
			if cmp == 0 {
				cmp = strings.Compare(a.ID, b.ID)
			}
			if q.Get("direction") == "desc" {
				return cmp > 0
			}
			return cmp < 0
		})
	}
	if out.Offset >= len(all) {
		return out, nil
	}
	end := out.Offset + out.Limit
	if end > len(all) {
		end = len(all)
	}
	ids := []string{}
	for _, f := range all[out.Offset:end] {
		ids = append(ids, f.ID)
		out.Items = append(out.Items, CatalogItem{Flow: f})
	}
	rows, e := s.DB.Query(ctx, `SELECT DISTINCT ON(b.flow_id) b.flow_id::text,b.id::text,COALESCE(b.error_code,b.engine_state),b.created_at,(SELECT count(*) FROM flow_execution_target t WHERE t.execution_id=b.id AND t.state='SUCCESS'),(SELECT count(*) FROM flow_execution_target t WHERE t.execution_id=b.id AND t.state='FAILED'),(SELECT count(*) FROM flow_execution_target t WHERE t.execution_id=b.id) FROM flow_execution_binding b WHERE b.flow_id::text=ANY($1) AND b.project_id=ANY($2) ORDER BY b.flow_id,b.created_at DESC,b.id DESC`, ids, p.Projects)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var flowID string
		v := &RecentExecution{}
		if e = rows.Scan(&flowID, &v.ID, &v.State, &v.CreatedAt, &v.Success, &v.Failed, &v.Total); e != nil {
			return out, e
		}
		for i := range out.Items {
			if out.Items[i].ID == flowID {
				out.Items[i].Recent = v
			}
		}
	}
	return out, rows.Err()
}

type ExecutionSummary struct {
	Total       int             `json:"total"`
	States      map[string]int  `json:"states"`
	Targets     map[string]int  `json:"targets"`
	Since       *time.Time      `json:"since"`
	Until       *time.Time      `json:"until"`
	StateSource string          `json:"state_source"`
	Duration    DurationSummary `json:"duration"`
	Daily       []DailySummary  `json:"daily"`
}
type DurationSummary struct {
	SampleCount  int      `json:"sample_count"`
	MissingCount int      `json:"missing_count"`
	Mean         *float64 `json:"mean_seconds"`
	P50          *float64 `json:"p50_seconds"`
	P95          *float64 `json:"p95_seconds"`
	Coverage     string   `json:"coverage"`
}
type DailySummary struct {
	Date   string `json:"date"`
	Total  int    `json:"total"`
	Failed int    `json:"failed"`
}

func durationSummary(values []float64, total int) DurationSummary {
	out := DurationSummary{SampleCount: len(values), MissingCount: total - len(values), Coverage: "Captured terminal engine state startDate/endDate only; uncaptured or incomplete executions excluded, not zero-filled. Empirical nearest-rank percentiles, no SLO inference."}
	if len(values) == 0 {
		return out
	}
	sort.Float64s(values)
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	out.Mean = &mean
	if len(values) > 1 {
		p50 := values[int(math.Ceil(float64(len(values))*0.5))-1]
		p95 := values[int(math.Ceil(float64(len(values))*0.95))-1]
		out.P50 = &p50
		out.P95 = &p95
	}
	return out
}

func queryTimes(q url.Values) (*time.Time, *time.Time, error) {
	var a, b *time.Time
	for k, d := range map[string]**time.Time{"since": &a, "until": &b} {
		if q.Get(k) != "" {
			v, e := time.Parse(time.RFC3339, q.Get(k))
			if e != nil {
				return nil, nil, problem(400, "EXECUTION_TIME_FILTER_INVALID")
			}
			*d = &v
		}
	}
	if a != nil && b != nil && a.After(*b) {
		return nil, nil, problem(400, "EXECUTION_TIME_FILTER_INVALID")
	}
	return a, b, nil
}
func (s *Service) ExecutionSummary(ctx context.Context, p Scope, q url.Values) (ExecutionSummary, error) {
	out := ExecutionSummary{States: map[string]int{}, Targets: map[string]int{}, Daily: []DailySummary{}, Duration: durationSummary(nil, 0), StateSource: "Persisted Kestra/target projection; not inferred from a loaded page"}
	var e error
	out.Since, out.Until, e = queryTimes(q)
	if e != nil {
		return out, e
	}
	revision, e := queryInt(q, "revision", 0, 1000000)
	if e != nil || q.Get("revision") != "" && revision < 1 {
		return out, problem(400, "FLOWS_REVISION_FILTER_INVALID")
	}
	selectedSQL := `SELECT id,created_at,COALESCE(error_code,engine_state) state FROM flow_execution_binding WHERE project_id=ANY($1) AND ($2='' OR flow_id::text=$2) AND ($3='' OR source_module=$3) AND ($4::timestamptz IS NULL OR created_at >= $4) AND ($5::timestamptz IS NULL OR created_at <= $5) AND ($6=0 OR flow_revision=$6)`
	args := []any{p.Projects, q.Get("flow_id"), q.Get("source"), out.Since, out.Until, revision}
	tx, e := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	rows, e := tx.Query(ctx, `WITH selected AS (`+selectedSQL+`) SELECT 'execution',state,count(*) FROM selected GROUP BY state UNION ALL SELECT 'target',t.state,count(*) FROM flow_execution_target t JOIN selected b ON t.execution_id=b.id GROUP BY t.state`, args...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var domain, state string
		var count int
		if e = rows.Scan(&domain, &state, &count); e != nil {
			return out, e
		}
		if domain == "execution" {
			out.States[state] = count
			out.Total += count
		} else {
			out.Targets[state] = count
		}
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	rows.Close()
	snapshots, e := tx.Query(ctx, `WITH selected AS (`+selectedSQL+`) SELECT b.created_at,b.state,e.payload->'state' FROM selected b LEFT JOIN LATERAL (SELECT payload FROM flow_event WHERE execution_id=b.id AND event_type='execution' ORDER BY id DESC LIMIT 1) e ON true`, args...)
	if e != nil {
		return out, e
	}
	defer snapshots.Close()
	durations := []float64{}
	days := map[string]*DailySummary{}
	for snapshots.Next() {
		var created time.Time
		var state string
		var rawState []byte
		if e = snapshots.Scan(&created, &state, &rawState); e != nil {
			return out, e
		}
		key := created.UTC().Format("2006-01-02")
		if days[key] == nil {
			days[key] = &DailySummary{Date: key}
		}
		days[key].Total++
		if state == "FAILED" {
			days[key].Failed++
		}
		var observed struct {
			Current string `json:"current"`
			Start   string `json:"startDate"`
			End     string `json:"endDate"`
		}
		if json.Unmarshal(rawState, &observed) != nil || !contains([]string{"SUCCESS", "FAILED", "WARNING", "KILLED", "CANCELLED"}, observed.Current) {
			continue
		}
		a, ea := time.Parse(time.RFC3339Nano, observed.Start)
		b, eb := time.Parse(time.RFC3339Nano, observed.End)
		if ea == nil && eb == nil && !b.Before(a) {
			durations = append(durations, b.Sub(a).Seconds())
		}
	}
	if e = snapshots.Err(); e != nil {
		return out, e
	}
	out.Duration = durationSummary(durations, out.Total)
	keys := []string{}
	for k := range days {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out.Daily = append(out.Daily, *days[k])
	}
	snapshots.Close()
	if e = tx.Commit(ctx); e != nil {
		return out, e
	}
	return out, nil
}

type LogSearchItem struct {
	ID          string `json:"id"`
	ExecutionID string `json:"execution_id"`
	Project     string `json:"project_id"`
	FlowID      string `json:"flow_id"`
	TaskID      string `json:"task_id"`
	TaskRunID   string `json:"task_run_id"`
	Level       string `json:"level"`
	Timestamp   string `json:"timestamp"`
	Message     string `json:"message"`
}
type LogSearchPage struct {
	Items    []LogSearchItem `json:"items"`
	Next     *string         `json:"next_cursor"`
	Coverage string          `json:"coverage"`
}

func (s *Service) LogSearch(ctx context.Context, p Scope, q url.Values) (LogSearchPage, error) {
	out := LogSearchPage{Items: []LogSearchItem{}, Coverage: "Captured, redacted execution log events only; uncaptured historical engine logs are not implied complete"}
	limit, e := queryInt(q, "limit", 50, 100)
	if e != nil || limit < 1 {
		return out, problem(400, "FLOWS_LIMIT_INVALID")
	}
	since, until, e := queryTimes(q)
	if e != nil {
		return out, e
	}
	scopeQ := url.Values{}
	for k, v := range q {
		if k != "cursor" {
			scopeQ[k] = v
		}
	}
	projects := append([]string{}, p.Projects...)
	sort.Strings(projects)
	digest := hash(p.User + strings.Join(projects, ",") + scopeQ.Encode())
	cursor := struct {
		ID    int64  `json:"id"`
		Scope string `json:"scope"`
	}{ID: 9223372036854775807, Scope: digest}
	if q.Get("cursor") != "" {
		b, e := base64.RawURLEncoding.DecodeString(q.Get("cursor"))
		if e != nil || json.Unmarshal(b, &cursor) != nil || cursor.Scope != digest {
			return out, problem(400, "FLOWS_CURSOR_SCOPE_INVALID")
		}
	}
	rows, e := s.DB.Query(ctx, `SELECT e.id::text,b.id::text,b.project_id,COALESCE(b.flow_id::text,''),COALESCE(e.payload->>'taskId',''),COALESCE(e.payload->>'taskRunId',''),COALESCE(e.payload->>'level',''),COALESCE(e.payload->>'timestamp',''),COALESCE(e.payload->>'message','') FROM flow_event e JOIN flow_execution_binding b ON b.id=e.execution_id WHERE b.project_id=ANY($1) AND e.event_type='log' AND e.id<$2 AND ($3='' OR b.flow_id::text=$3) AND ($4='' OR b.id::text=$4) AND ($5='' OR e.payload->>'taskId'=$5) AND ($6='' OR e.payload->>'level'=$6) AND ($7='' OR e.payload->>'message' ILIKE '%'||$7||'%') AND ($8::timestamptz IS NULL OR e.created_at >= $8) AND ($9::timestamptz IS NULL OR e.created_at <= $9) ORDER BY e.id DESC LIMIT $10`, p.Projects, cursor.ID, q.Get("flow_id"), q.Get("execution_id"), q.Get("task_id"), q.Get("level"), q.Get("q"), since, until, limit+1)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var v LogSearchItem
		if e = rows.Scan(&v.ID, &v.ExecutionID, &v.Project, &v.FlowID, &v.TaskID, &v.TaskRunID, &v.Level, &v.Timestamp, &v.Message); e != nil {
			return out, e
		}
		v.Message = redactOutput(v.Message)
		out.Items = append(out.Items, v)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
		cursor.ID, _ = strconv.ParseInt(out.Items[len(out.Items)-1].ID, 10, 64)
		v := base64.RawURLEncoding.EncodeToString(raw(cursor))
		out.Next = &v
	}
	return out, nil
}

type ExecutionReference struct {
	ID        string    `json:"id"`
	Relation  string    `json:"relation"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
}
type ExecutionInsights struct {
	Previous     *string              `json:"previous_id"`
	Next         *string              `json:"next_id"`
	Relations    []ExecutionReference `json:"relations"`
	Metrics      []map[string]any     `json:"metrics"`
	MetricsTotal int                  `json:"metrics_total"`
	MetricsError *string              `json:"metrics_error"`
}

func (s *Service) ExecutionInsights(ctx context.Context, p Scope, id string, q url.Values) (ExecutionInsights, error) {
	out := ExecutionInsights{Relations: []ExecutionReference{}, Metrics: []map[string]any{}}
	ex, e := s.Execution(ctx, p, id)
	if e != nil {
		return out, e
	}
	page, e := queryInt(q, "page", 1, 100000)
	if e != nil || page < 1 {
		return out, problem(400, "FLOWS_PAGINATION_INVALID")
	}
	limit, e := queryInt(q, "limit", 50, 100)
	if e != nil || limit < 1 {
		return out, problem(400, "FLOWS_LIMIT_INVALID")
	}
	rows, e := s.DB.Query(ctx, `SELECT id::text,CASE WHEN id=$2::uuid THEN 'parent' ELSE 'child' END,COALESCE(error_code,engine_state),created_at FROM flow_execution_binding WHERE project_id=ANY($1) AND (id=$2::uuid OR parent_id::text=$3) ORDER BY created_at,id`, p.Projects, ex.ParentID, id)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var v ExecutionReference
		if e = rows.Scan(&v.ID, &v.Relation, &v.State, &v.CreatedAt); e != nil {
			rows.Close()
			return out, e
		}
		out.Relations = append(out.Relations, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, before := range []bool{true, false} {
		query := `SELECT x.id::text FROM flow_execution_binding x JOIN flow_execution_binding current ON current.id::text=$1 WHERE x.project_id=ANY($2) AND x.project_id=current.project_id AND x.namespace=current.namespace AND x.engine_flow_id=current.engine_flow_id AND (x.created_at,x.id)`
		if before {
			query += `<(current.created_at,current.id) ORDER BY x.created_at DESC,x.id DESC LIMIT 1`
		} else {
			query += `>(current.created_at,current.id) ORDER BY x.created_at,x.id LIMIT 1`
		}
		var target *string
		err := s.DB.QueryRow(ctx, query, id, p.Projects).Scan(&target)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return out, err
		}
		if err == nil {
			if before {
				out.Previous = target
			} else {
				out.Next = target
			}
		}
	}
	if ex.KestraID != nil && q.Get("metrics") != "false" {
		var metrics struct {
			Results []map[string]any `json:"results"`
			Total   int              `json:"total"`
		}
		params := url.Values{"page": {strconv.Itoa(page)}, "size": {strconv.Itoa(limit)}}
		if q.Get("task_id") != "" {
			params.Set("taskId", q.Get("task_id"))
		}
		if e = s.Engine.JSON(ctx, "GET", "/metrics/"+url.PathEscape(*ex.KestraID)+"?"+params.Encode(), &metrics); e != nil {
			v := "METRICS_UNAVAILABLE"
			out.MetricsError = &v
		} else {
			if metrics.Results != nil {
				out.Metrics = metrics.Results
			}
			out.MetricsTotal = metrics.Total
		}
	}
	if ex.KestraID == nil && q.Get("metrics") != "false" {
		v := "EXECUTION_ENGINE_ID_PENDING"
		out.MetricsError = &v
	}
	return out, nil
}
func (s *Service) TaskDocumentation(ctx context.Context, p Scope, kind string) (map[string]any, error) {
	ns := "keel.docs"
	source := fmt.Sprintf("id: docs\nnamespace: %s\ntasks:\n  - id: selected\n    type: %s\n", ns, kind)
	if !strings.HasPrefix(kind, "io.kestra.") || strings.ContainsAny(kind, "\r\n /?") {
		return nil, problem(400, "TASK_TYPE_INVALID")
	}
	if e := ValidatePolicy(source, ns); e != nil {
		return nil, e
	}
	var doc map[string]any
	if e := s.Engine.JSON(ctx, "GET", "/plugins/"+url.PathEscape(kind), &doc); e != nil {
		return nil, e
	}
	return map[string]any{"type": kind, "documentation": doc, "policy": "Engine policy constrained; device scripts use scoped Job Service"}, nil
}

func scrubVariables(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, child := range v {
			key := strings.ToLower(k)
			if strings.Contains(key, "password") || strings.Contains(key, "secret") || strings.Contains(key, "token") || strings.Contains(key, "credential") || strings.Contains(key, "private_key") || key == "ticket" || key == "apikey" || key == "api_key" {
				out[k] = "[REDACTED]"
			} else {
				out[k] = scrubVariables(child)
			}
		}
		return out
	case []any:
		out := []any{}
		for _, child := range v {
			out = append(out, scrubVariables(child))
		}
		return out
	case string:
		return redactOutput(v)
	default:
		return v
	}
}
func (s *Service) ExecutionVariables(ctx context.Context, p Scope, id string) (map[string]any, error) {
	ex, e := s.Execution(ctx, p, id)
	if e != nil {
		return nil, e
	}
	inputs, _ := ex.Engine["inputs"].(map[string]any)
	outputs, _ := ex.Engine["outputs"].(map[string]any)
	if inputs == nil {
		inputs = map[string]any{}
	}
	if outputs == nil {
		outputs = map[string]any{}
	}
	definition, err := s.ExecutionDefinition(ctx, p, id)
	if err != nil {
		for key := range inputs {
			inputs[key] = "[REDACTED: input schema unavailable]"
		}
	} else if doc, e := FlowDocument(definition.Source); e == nil {
		definitions, _ := doc["inputs"].([]any)
		for _, item := range definitions {
			field, _ := item.(map[string]any)
			if field["type"] == "SECRET" {
				key := fmt.Sprint(field["id"])
				if _, exists := inputs[key]; exists {
					inputs[key] = "[REDACTED]"
				}
			}
		}
	} else {
		for key := range inputs {
			inputs[key] = "[REDACTED: input schema invalid]"
		}
	}
	return map[string]any{"inputs": scrubVariables(inputs), "outputs": scrubVariables(outputs)}, nil
}
