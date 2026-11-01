package engineering

import (
	"fmt"
	"math/big"
	"sort"
)

// CableRoutingEdge is an explicitly permitted channel, never an inferred
// straight line between rack coordinates. Distance is supplied or calculated
// by an approved project rule before route selection.
type CableRoutingEdge struct {
	FromID   ID
	ToID     ID
	Medium   string
	Distance Quantity
}

func ShortestCablePath(nodes []ID, edges []CableRoutingEdge, start, end ID, medium string) ([]RouteSegment, Quantity, error) {
	available := map[ID]bool{}
	for _, id := range nodes {
		if id == "" || available[id] {
			return nil, Quantity{}, fmt.Errorf("CABLE_NODE_INVALID")
		}
		available[id] = true
	}
	if !available[start] || !available[end] || start == end || medium == "" {
		return nil, Quantity{}, fmt.Errorf("CABLE_ENDPOINT_UNRESOLVED")
	}
	type arc struct {
		to       ID
		distance *big.Rat
	}
	graph := map[ID][]arc{}
	for _, edge := range edges {
		if !available[edge.FromID] || !available[edge.ToID] || edge.FromID == edge.ToID {
			return nil, Quantity{}, fmt.Errorf("CABLE_EDGE_INVALID")
		}
		v, e := measure(edge.Distance)
		if e != nil || v.dimension != "length" || v.value.Sign() <= 0 {
			return nil, Quantity{}, fmt.Errorf("CABLE_EDGE_DISTANCE_INVALID")
		}
		if edge.Medium != medium && edge.Medium != "*" {
			continue
		}
		graph[edge.FromID] = append(graph[edge.FromID], arc{edge.ToID, v.value})
		graph[edge.ToID] = append(graph[edge.ToID], arc{edge.FromID, v.value})
	}
	distances := map[ID]*big.Rat{start: new(big.Rat)}
	visited := map[ID]bool{}
	previous := map[ID]ID{}
	weights := map[ID]*big.Rat{}
	ordered := append([]ID{}, nodes...)
	sort.Strings(ordered)
	for {
		current := ID("")
		for _, id := range ordered {
			if !visited[id] && distances[id] != nil && (current == "" || distances[id].Cmp(distances[current]) < 0) {
				current = id
			}
		}
		if current == "" {
			return nil, Quantity{}, fmt.Errorf("CABLE_ROUTE_UNAVAILABLE")
		}
		if current == end {
			break
		}
		visited[current] = true
		for _, edge := range graph[current] {
			if visited[edge.to] {
				continue
			}
			candidate := new(big.Rat).Add(distances[current], edge.distance)
			if distances[edge.to] == nil || candidate.Cmp(distances[edge.to]) < 0 {
				distances[edge.to] = candidate
				previous[edge.to] = current
				weights[edge.to] = new(big.Rat).Set(edge.distance)
			}
		}
	}
	path := []RouteSegment{}
	quantity := func(v *big.Rat) Quantity {
		result, _ := resultValue(measured{v, "length"}, "m")
		return Quantity{Value: *result.Decimal, Unit: "m"}
	}
	for current := end; current != start; current = previous[current] {
		q := quantity(weights[current])
		path = append(path, RouteSegment{FromID: previous[current], ToID: current, Distance: &q})
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path, quantity(distances[end]), nil
}

// ApplyCableAllowance is the optional multiplicative project rule kernel.
// Callers must choose and approve the policy; there are no built-in lead or
// slack values and the result never represents an as-built length.
func ApplyCableAllowance(route Quantity, leads []Quantity, factor string) (Quantity, error) {
	total, e := measure(route)
	if e != nil || total.dimension != "length" || total.value.Sign() < 0 {
		return Quantity{}, fmt.Errorf("CABLE_LENGTH_INVALID")
	}
	for _, lead := range leads {
		v, e := measure(lead)
		if e != nil || v.dimension != "length" || v.value.Sign() < 0 {
			return Quantity{}, fmt.Errorf("CABLE_LEAD_INVALID")
		}
		total.value.Add(total.value, v.value)
	}
	if !decimal(factor) {
		return Quantity{}, fmt.Errorf("CABLE_ALLOWANCE_INVALID")
	}
	n, _ := new(big.Rat).SetString(factor)
	if n.Cmp(big.NewRat(1, 1)) < 0 {
		return Quantity{}, fmt.Errorf("CABLE_ALLOWANCE_INVALID")
	}
	total.value.Mul(total.value, n)
	v, e := resultValue(total, "m")
	if e != nil {
		return Quantity{}, e
	}
	return Quantity{Value: *v.Decimal, Unit: "m"}, nil
}
func SelectCableStock(required Quantity, stock []Quantity) (Quantity, error) {
	need, e := measure(required)
	if e != nil || need.dimension != "length" || need.value.Sign() <= 0 {
		return Quantity{}, fmt.Errorf("CABLE_LENGTH_INVALID")
	}
	chosen := -1
	var length *big.Rat
	for i, q := range stock {
		v, e := measure(q)
		if e != nil || v.dimension != "length" || v.value.Sign() <= 0 {
			return Quantity{}, fmt.Errorf("CABLE_STOCK_INVALID")
		}
		if v.value.Cmp(need.value) >= 0 && (chosen < 0 || v.value.Cmp(length) < 0) {
			chosen = i
			length = v.value
		}
	}
	if chosen < 0 {
		return Quantity{}, fmt.Errorf("CABLE_STOCK_UNAVAILABLE")
	}
	return stock[chosen], nil
}
