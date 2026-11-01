package engineering

import "testing"

func TestCableRoutingIndependentFixtureExpectation(t *testing.T) {
	nodes := []ID{"A", "M", "B"}
	edges := []CableRoutingEdge{{"A", "M", "fiber", Quantity{"500", "cm"}}, {"M", "B", "fiber", Quantity{"7", "m"}}, {"A", "B", "fiber", Quantity{"30", "m"}}, {"A", "B", "copper", Quantity{"1", "m"}}}
	path, route, e := ShortestCablePath(nodes, edges, "A", "B", "fiber")
	if e != nil {
		t.Fatal(e)
	}
	if len(path) != 2 || path[0].FromID != "A" || path[0].ToID != "M" || route.Value != "12" || route.Unit != "m" {
		t.Fatal("wrong permitted route", path, route)
	}
	length, e := ApplyCableAllowance(route, []Quantity{{"2", "m"}, {"2000", "mm"}}, "1.1")
	if e != nil || length.Value != "17.6" {
		t.Fatal("wrong engineering calculation", length, e)
	}
	stock, e := SelectCableStock(length, []Quantity{{"30", "m"}, {"1000", "cm"}, {"20", "m"}})
	if e != nil || stock.Value != "20" {
		t.Fatal("wrong stock selection", stock, e)
	}
	if _, _, e = ShortestCablePath(nodes, edges, "A", "B", "unsupported"); e == nil {
		t.Fatal("invented route")
	}
	if _, e = SelectCableStock(length, []Quantity{{"10", "m"}}); e == nil {
		t.Fatal("invented stock length")
	}
	if _, e = ApplyCableAllowance(route, []Quantity{{"1", "W"}}, "1.1"); e == nil {
		t.Fatal("accepted wrong units")
	}
}
