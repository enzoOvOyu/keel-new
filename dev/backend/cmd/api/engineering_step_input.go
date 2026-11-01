package main

import (
	"bytes"
	"encoding/csv"
	"keel.local/dev/internal/engineering"
	"net/http"
	"strings"
)

func (s *Server) engineeringStepInputRoutes(w http.ResponseWriter, r *http.Request) bool {
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(p) < 5 || p[0] != "api" || p[1] != "v1" || p[2] != "projects" || p[4] != "engineering-inputs" {
		return false
	}
	permission := "bom:read"
	if r.Method != "GET" {
		permission = "bom:edit"
	}
	if !subjectOf(r.Context()).Can(permission, p[3], "", "") {
		fail(w, 403, "FORBIDDEN")
		return true
	}
	if len(p) == 5 && r.Method == "GET" {
		respond(w, 200, engineering.StepInputCatalog())
		return true
	}
	if len(p) == 6 && p[5] == "preview" && r.Method == "POST" {
		var q engineering.InputPreviewRequest
		if engineeringDecode(w, r, &q) {
			respond(w, 200, engineering.PreviewStepInput(q))
		}
		return true
	}
	if len(p) == 6 && r.Method == "GET" {
		for _, t := range engineering.StepInputCatalog().Tables {
			if t.ID != p[5] {
				continue
			}
			headers := []string{}
			for _, f := range t.Fields {
				headers = append(headers, f.Label)
			}
			format := r.URL.Query().Get("format")
			if format == "" || format == "csv" {
				var body bytes.Buffer
				body.WriteString("\ufeff")
				writer := csv.NewWriter(&body)
				if e := writer.Write(headers); e != nil {
					fail(w, 500, "TEMPLATE_FAILED")
					return true
				}
				writer.Flush()
				w.Header().Set("Content-Type", "text/csv; charset=utf-8")
				w.Header().Set("Content-Disposition", `attachment; filename="`+t.ID+`.csv"`)
				w.Write(body.Bytes())
				return true
			}
			if format != "xlsx" {
				fail(w, 400, "INVALID_FORMAT")
				return true
			}
			body, e := engineering.WriteStepWorkbook(t.Step, []engineering.StandardTableData{{TableID: t.ID}})
			if e != nil {
				fail(w, 500, "TEMPLATE_FAILED")
				return true
			}

			w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
			w.Header().Set("Content-Disposition", `attachment; filename="`+t.ID+`.xlsx"`)
			w.Write(body)
			return true
		}
		fail(w, 404, "NOT_FOUND")
		return true
	}
	fail(w, 405, "METHOD_NOT_ALLOWED")
	return true
}
