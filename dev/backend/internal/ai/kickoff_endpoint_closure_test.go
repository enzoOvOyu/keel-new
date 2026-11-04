package ai

import (
	"fmt"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/engineering"
	"testing"
)

func TestKickoffEndpointIdentityWithoutSeparateInventory(t *testing.T) {
	sheet := func(name string, rows [][]string) bom.Sheet {
		s := bom.Sheet{Name: name}
		for ri, row := range rows {
			for ci, v := range row {
				s.Cells = append(s.Cells, bom.Cell{Sheet: name, Row: ri + 1, Column: ci + 1, Ref: fmt.Sprintf("%c%d", 'A'+ci, ri+1), Value: v})
			}
		}
		return s
	}
	wb := bom.Workbook{Sheets: []bom.Sheet{sheet("links", [][]string{{"Local name", "Local SN", "Local port", "Remote name", "Remote SN", "Remote port", "Length"}, {"host-a", "SYN-A", "p1", "switch-b", "SYN-B", "p2", "25"}}), sheet("locations", [][]string{{"Name", "Rack No", "U Position"}, {"host-a", "rack-a", "47u-"}})}}
	bounds := engineering.Bounds{StartRow: 1, EndRow: 2, StartColumn: 1, EndColumn: 7}
	regions := []KickoffRegion{{ID: "links", Sheet: "links", Bounds: bounds}, {ID: "positions", Sheet: "locations", Bounds: engineering.Bounds{StartRow: 1, EndRow: 2, StartColumn: 1, EndColumn: 3}}}
	relation := KickoffRelation{ID: "local", Kind: "device_identity", Source: KickoffRelationSource{"links", 1, 2, 2}, Targets: []KickoffRelationTarget{{"links", []int{1}, 2, 2, 2}}, Reason: "Explicit SN, name and port identify the local endpoint; placement corroborates its label", Evidence: []KickoffCellRef{{"links", "B2"}, {"locations", "A2"}}}
	plan := KickoffRelationshipPlan{Entities: []KickoffEntityAssessment{{Region: "links", Kind: "connection"}}, Relations: []KickoffRelation{relation}}
	index, _, err := evaluateKickoffRelations(wb, plan)
	if err != nil {
		t.Fatal(err)
	}
	mapping := KickoffMapping{Region: "links", Table: "kickoff23/devices", Bounds: engineering.Bounds{StartRow: 2, EndRow: 2, StartColumn: 1, EndColumn: 7}, Layout: "rows", KeyColumn: 2, Bindings: []KickoffBinding{{Field: "serial", Column: 1, Relationship: "local"}, {Field: "label", Column: 1}}}
	if err = validateKickoffRelationshipBindings(wb, regions, plan, KickoffTaskResult{Mappings: []KickoffMapping{mapping}}); err != nil {
		t.Fatal(err)
	}
	rows, proof, err := expandKickoffRegionWithRelationships(wb, regions[0], mapping, index)
	if err != nil {
		t.Fatal(err)
	}
	c := kickoffAgentCandidate(mapping.Table, rows, proof)
	if c.Rows[0]["serial"] != "SYN-A" {
		t.Fatal("explicit endpoint SN lost")
	}
	relation.ID = "position"
	relation.Source = KickoffRelationSource{"locations", 1, 2, 2}
	plan.Relations = []KickoffRelation{relation}
	index, _, err = evaluateKickoffRelations(wb, plan)
	if err != nil {
		t.Fatal(err)
	}
	mapping = KickoffMapping{Region: "positions", Table: "kickoff23/placements", Bounds: engineering.Bounds{StartRow: 2, EndRow: 2, StartColumn: 1, EndColumn: 3}, Layout: "rows", KeyColumn: 1, Bindings: []KickoffBinding{{Field: "device_id", Column: 1, Relationship: "position"}, {Field: "rack_id", Column: 2}, {Field: "start_u", Column: 3}}}
	if err = validateKickoffRelationshipBindings(wb, regions, plan, KickoffTaskResult{Mappings: []KickoffMapping{mapping}}); err != nil {
		t.Fatal(err)
	}
	rows, proof, err = expandKickoffRegionWithRelationships(wb, regions[1], mapping, index)
	if err != nil {
		t.Fatal(err)
	}
	c = kickoffAgentCandidate(mapping.Table, rows, proof)
	if c.Rows[0]["device_id"] != "SYN-A" || c.Rows[0]["start_u"] != "47" || len(c.Issues) > 0 {
		t.Fatal("placement did not close on explicit endpoint SN and U")
	}
}
