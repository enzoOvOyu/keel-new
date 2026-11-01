package main

import (
	"errors"
	"keel.local/dev/internal/engineering"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func calculationFailure(w http.ResponseWriter, e error) {
	code := e.Error()
	if strings.HasPrefix(code, "RULE_") || strings.HasPrefix(code, "CALC_") || code == "WORKBOOK_PARAMETER_BINDINGS_REQUIRED" {
		fail(w, 422, code)
		return
	}
	engineeringFailure(w, e)
}
func (s *Server) engineeringCalculationRoutes(w http.ResponseWriter, r *http.Request) bool {
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(p) < 5 || p[0] != "api" || p[1] != "v1" || p[2] != "projects" {
		return false
	}
	rules := p[4] == "engineering-rules"
	calcs := p[4] == "engineering-imports" && len(p) >= 7 && (p[6] == "calculations" || p[6] == "calculation-provider")
	if !rules && !calcs {
		return false
	}
	project := p[3]
	permission := "bom:read"
	if r.Method != "GET" {
		permission = "bom:edit"
		if rules && len(p) == 8 && p[7] == "review" {
			permission = "bom:approve"
		}
	}
	if !subjectOf(r.Context()).Can(permission, project, "", "") {
		fail(w, 403, "FORBIDDEN")
		return true
	}
	scoped := *s
	scoped.project = project
	s = &scoped
	ctx := r.Context()
	actor := subjectOf(ctx).Username
	if rules {
		if len(p) == 5 && r.Method == "GET" {
			rows, e := s.db.Query(ctx, `SELECT id::text,version FROM eng_rule WHERE project_id=$1 ORDER BY created_at DESC LIMIT 100`, project)
			if e != nil {
				calculationFailure(w, e)
				return true
			}
			refs := []engineering.VersionReference{}
			for rows.Next() {
				var ref engineering.VersionReference
				if e = rows.Scan(&ref.ID, &ref.Version); e != nil {
					break
				}
				refs = append(refs, ref)
			}
			if e == nil {
				e = rows.Err()
			}
			rows.Close()
			if e != nil {
				calculationFailure(w, e)
				return true
			}
			views := []engineering.RuleView{}
			for _, ref := range refs {
				v, err := engineeringRule(ctx, s.db, project, ref.ID, ref.Version)
				if err != nil {
					calculationFailure(w, err)
					return true
				}
				views = append(views, v)
			}
			respond(w, 200, views)
			return true
		}
		if len(p) == 5 && r.Method == "POST" {
			var q engineering.RuleDraft
			if !engineeringDecode(w, r, &q) {
				return true
			}
			v, e := s.saveEngineeringRule(ctx, q, actor)
			if e != nil {
				calculationFailure(w, e)
			} else {
				respond(w, 201, v)
			}
			return true
		}
		if len(p) == 7 || len(p) == 8 {
			version, e := strconv.Atoi(p[6])
			if e != nil || version < 1 {
				calculationFailure(w, errors.New("ENGINEERING_SCHEMA_INVALID"))
				return true
			}
			if len(p) == 7 && r.Method == "GET" {
				v, e := engineeringRule(ctx, s.db, project, p[5], version)
				if e != nil {
					calculationFailure(w, e)
				} else {
					respond(w, 200, v)
				}
				return true
			}
			if len(p) == 8 && p[7] == "review" && r.Method == "POST" {
				var q engineering.RuleReview
				if !engineeringDecode(w, r, &q) {
					return true
				}
				v, e := s.reviewEngineeringRule(ctx, p[5], version, q, actor)
				if e != nil {
					calculationFailure(w, e)
				} else {
					respond(w, 200, v)
				}
				return true
			}
		}
	} else {
		source := p[5]
		workspace, e := s.engineeringWorkspace(ctx, source)
		if e != nil {
			calculationFailure(w, e)
			return true
		}
		if len(p) == 7 && p[6] == "calculation-provider" && r.Method == "GET" {
			caps, _ := (engineering.LibreOfficeProvider{Image: os.Getenv("KEEL_CALC_IMAGE")}).Capabilities(ctx)
			respond(w, 200, caps)
			return true
		}
		if p[6] == "calculations" && len(p) == 7 {
			if r.Method == "POST" {
				var q engineering.CalculationRequest
				if !engineeringDecode(w, r, &q) {
					return true
				}
				id, e := s.queueEngineeringCalculation(ctx, source, q, actor)
				if e != nil {
					calculationFailure(w, e)
					return true
				}
				v, e := s.engineeringCalculation(ctx, id, 0, 100)
				if e != nil {
					calculationFailure(w, e)
				} else {
					respond(w, 202, v)
				}
				return true
			}
			if r.Method == "GET" {
				rows, e := s.db.Query(ctx, `SELECT id::text FROM eng_calculation_run WHERE source_id=$1 AND project_id=$2 ORDER BY created_at DESC LIMIT 30`, source, project)
				if e != nil {
					calculationFailure(w, e)
					return true
				}
				ids := []string{}
				for rows.Next() {
					var id string
					if e = rows.Scan(&id); e != nil {
						break
					}
					ids = append(ids, id)
				}
				if e == nil {
					e = rows.Err()
				}
				rows.Close()
				if e != nil {
					calculationFailure(w, e)
					return true
				}
				views := []engineering.CalculationPage{}
				for _, id := range ids {
					v, err := s.engineeringCalculation(ctx, id, 0, 0)
					if err != nil {
						calculationFailure(w, err)
						return true
					}
					views = append(views, v)
				}
				respond(w, 200, views)
				return true
			}
		}
		if p[6] == "calculations" && len(p) == 8 && r.Method == "GET" {
			var found bool
			if e = s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM eng_calculation_run WHERE id::text=$1 AND source_id=$2 AND project_id=$3)`, p[7], workspace.ID, project).Scan(&found); e != nil {
				calculationFailure(w, e)
				return true
			}
			if !found {
				fail(w, 404, "NOT_FOUND")
				return true
			}
			offset, limit := 0, 100
			if value := r.URL.Query().Get("offset"); value != "" {
				offset, e = strconv.Atoi(value)
			}
			if e == nil {
				if value := r.URL.Query().Get("limit"); value != "" {
					limit, e = strconv.Atoi(value)
				}
			}
			if e != nil || offset < 0 || limit < 1 || limit > 200 {
				calculationFailure(w, errors.New("CALC_REQUEST_LIMIT"))
				return true
			}
			v, e := s.engineeringCalculation(ctx, p[7], offset, limit)
			if e != nil {
				calculationFailure(w, e)
			} else {
				respond(w, 200, v)
			}
			return true
		}
	}
	fail(w, 404, "NOT_FOUND")
	return true
}
