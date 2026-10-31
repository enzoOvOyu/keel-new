package flows

import (
	"context"
	"fmt"
	"net/url"
)

func (s *Service) Dependencies(ctx context.Context, p Scope, id string) (map[string]any, error) {
	f, e := s.Flow(ctx, p, id)
	if e != nil {
		return nil, e
	}
	var graph map[string]any
	if e = s.Engine.JSON(ctx, "GET", "/flows/"+url.PathEscape(f.Namespace)+"/"+url.PathEscape(f.FlowID)+"/dependencies", &graph); e != nil {
		return nil, e
	}
	bindings, e := s.Flows(ctx, p)
	if e != nil {
		return nil, e
	}
	registered := map[string]string{}
	for _, b := range bindings {
		registered[b.Namespace+":"+b.FlowID] = b.ID
	}
	allowed := map[string]bool{}
	nodes, edges := []map[string]any{}, []map[string]any{}
	partial := false
	rawNodes, _ := graph["nodes"].([]any)
	for _, v := range rawNodes {
		n, _ := v.(map[string]any)
		ns, _ := n["namespace"].(string)
		fid, _ := n["id"].(string)
		binding := registered[ns+":"+fid]
		if binding == "" || ns != f.Namespace {
			partial = true
			continue
		}
		uid := fmt.Sprint(n["uid"])
		allowed[uid] = true
		nodes = append(nodes, map[string]any{"uid": uid, "namespace": ns, "flow_id": fid, "binding_id": binding})
	}
	rawEdges, _ := graph["edges"].([]any)
	for _, v := range rawEdges {
		edge, _ := v.(map[string]any)
		if allowed[fmt.Sprint(edge["source"])] && allowed[fmt.Sprint(edge["target"])] {
			edges = append(edges, edge)
		}
	}
	return map[string]any{"nodes": nodes, "edges": edges, "partial": partial}, nil
}
