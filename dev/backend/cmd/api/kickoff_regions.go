package main

import (
	"context"
	"fmt"
	"github.com/xuri/excelize/v2"
	"keel.local/dev/internal/engineering"
	"strings"
)

type kickoffRegion struct {
	ID       string `json:"id"`
	Sheet    string `json:"sheet"`
	Range    string `json:"range"`
	Category string `json:"category"`
	RowCount int    `json:"rowCount"`
	State    string `json:"state"`
	Note     string `json:"note"`
}

func kickoffCategory(semantic string) string {
	value := strings.ToLower(semantic)
	for _, entry := range []struct {
		words    []string
		category string
	}{{[]string{"connection", "connectivity", "端接", "连线", "链路"}, "物理连接"}, {[]string{"component", "assembly", "模块", "组件"}, "组件关系"}, {[]string{"port", "端口"}, "端口"}, {[]string{"placement", "落位"}, "落位"}, {[]string{"rack", "facility", "机柜", "机房"}, "机房机柜"}, {[]string{"equipment", "device", "设备"}, "设备"}} {
		for _, word := range entry.words {
			if strings.Contains(value, word) {
				return entry.category
			}
		}
	}
	return "待分类"
}
func loadKickoffRegions(ctx context.Context, q engineeringQuery, source string) ([]kickoffRegion, error) {
	out := []kickoffRegion{}
	rows, e := q.Query(ctx, `SELECT r.id::text,r.sheet,r.start_row,r.end_row,r.start_column,r.end_column,r.semantic_label FROM eng_workbook_region r JOIN eng_source s ON s.head_revision_id=r.revision_id WHERE s.id::text=$1 ORDER BY r.sheet,r.start_row,r.start_column`, source)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var v kickoffRegion
		var sr, er, sc, ec int
		var semantic string
		if e = rows.Scan(&v.ID, &v.Sheet, &sr, &er, &sc, &ec, &semantic); e != nil {
			break
		}
		a, err := excelize.CoordinatesToCellName(sc, sr)
		if err != nil {
			e = err
			break
		}
		b, err := excelize.CoordinatesToCellName(ec, er)
		if err != nil {
			e = err
			break
		}
		v.Range = a + ":" + b
		v.RowCount = er - sr + 1
		v.Category = kickoffCategory(semantic)
		v.State = "extracted"
		if v.Category == "待分类" {
			v.State = "unmapped"
		}
		v.Note = semantic
		out = append(out, v)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return nil, e
	}
	if len(out) > 0 {
		return out, nil
	}
	// A historical typed candidate can predate region metadata. Retain its actual
	// workbook sheet extents, clearly unclassified; do not invent analysis results.
	rows, e = q.Query(ctx, `SELECT sh->>'sheet_name',COALESCE(sh->>'dimension',''),(SELECT count(DISTINCT c->>'source_row') FROM jsonb_array_elements(COALESCE(sh->'source_cells','[]')) c WHERE COALESCE(c->>'original_value','')<>'') FROM bom_import_batch b JOIN eng_source s ON s.import_id=b.id LEFT JOIN kickoff_history_import h ON h.source_id=s.id LEFT JOIN eng_source original ON original.id=h.original_source_id LEFT JOIN bom_import_batch original_batch ON original_batch.id=original.import_id CROSS JOIN LATERAL jsonb_array_elements(COALESCE(original_batch.workbook->'sheets',b.workbook->'sheets','[]')) sh WHERE s.id::text=$1`, source)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		v := kickoffRegion{Category: "待分类", State: "unmapped", Note: "历史工作表范围；语义分类待补充"}
		if e = rows.Scan(&v.Sheet, &v.Range, &v.RowCount); e != nil {
			return nil, e
		}
		v.ID = engineering.StableID(source, "sheet-range", v.Sheet)
		if v.Range == "" {
			v.Range = fmt.Sprintf("%d个非空行", v.RowCount)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
