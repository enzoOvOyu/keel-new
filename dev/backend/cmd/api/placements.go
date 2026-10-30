package main

import (
	"encoding/json"
	"keel.local/dev/internal/bom"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) placements(w http.ResponseWriter, r *http.Request, id string, rev int, status string, workbook, draft []byte) bool {
	if r.Method != "PATCH" || !strings.HasSuffix(r.URL.Path, "/placements") {
		return false
	}
	if status == "COMMITTED" {
		fail(w, 409, "IMPORT_IMMUTABLE")
		return true
	}
	var q struct {
		Revision int    `json:"revision"`
		Item     string `json:"item_uid"`
		Site     string `json:"site"`
		Rack     string `json:"rack"`
		U        int    `json:"u_start"`
		Height   int    `json:"u_height"`
		Capacity int    `json:"rack_height"`
		Sheet    string `json:"sheet_name"`
		Cell     string `json:"source_cell"`
		Reason   string `json:"reason"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.Revision != rev {
		fail(w, 409, "REVISION_CONFLICT")
		return true
	}
	if q.Site == "" || q.Rack == "" || q.Reason == "" || q.U < 1 || q.Height < 1 || q.Capacity < q.U+q.Height-1 {
		fail(w, 422, "CONFIRMED_PLACEMENT_REQUIRED")
		return true
	}
	var wb bom.Workbook
	var items []bom.Item
	json.Unmarshal(workbook, &wb)
	json.Unmarshal(draft, &items)
	var source *bom.Cell
	for _, sh := range wb.Sheets {
		if sh.Name == q.Sheet {
			for _, c := range sh.Cells {
				if c.Ref == q.Cell && !c.Sensitive {
					copy := c
					source = &copy
				}
			}
		}
	}
	if source == nil {
		fail(w, 422, "SOURCE_REFERENCE_INVALID")
		return true
	}
	found := false
	for j := range items {
		item := &items[j]
		if item.UID != q.Item {
			continue
		}
		if item.Category != "server" && item.Category != "switch" {
			fail(w, 422, "PLACEMENT_DEVICE_REQUIRED")
			return true
		}
		found = true
		if item.Fields == nil {
			item.Fields = map[string]string{}
		}
		if item.Sources == nil {
			item.Sources = map[string]bom.Cell{}
		}
		item.Fields["site"] = q.Site
		item.Fields["rack"] = q.Rack
		item.Fields["u_start"] = strconv.Itoa(q.U)
		item.Fields["u_height"] = strconv.Itoa(q.Height)
		item.Fields["rack_height"] = strconv.Itoa(q.Capacity)
		item.Sources["manual_position@"+q.Sheet+":"+q.Cell] = *source
	}
	if !found {
		fail(w, 404, "NOT_FOUND")
		return true
	}
	items = bom.EnsureRacks(items)
	raw, _ := json.Marshal(items)
	if e := s.mutate(r.Context(), id, "placement.human_confirmed", q.Reason, "UPDATE bom_import_batch SET draft=$2,revision=revision+1,reviewed_revision=NULL,status='MAPPING' WHERE id=$1 AND revision=$3", id, raw, rev); e != nil {
		fail(w, 409, "REVISION_CONFLICT")
		return true
	}
	respond(w, 200, map[string]int{"revision": rev + 1})
	return true
}
