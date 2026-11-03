package main

import (
	"context"
	"encoding/json"
	"fmt"
)

// A receipt only covers a repeat when both the immutable engineering revision
// and every input version match. A changed revision/input must remain publishable.
func kickoffScopeCovered(priorIDs, wanted []string, priorVersions, versions map[string]int) bool {
	if len(wanted) == 0 || len(priorVersions) != len(versions) {
		return false
	}
	for key, version := range versions {
		if prior, ok := priorVersions[key]; !ok || prior != version {
			return false
		}
	}
	ids := map[string]bool{}
	for _, id := range priorIDs {
		ids[id] = true
	}
	for _, id := range wanted {
		if !ids[id] {
			return false
		}
	}
	return true
}
func kickoffCoveredPublication(ctx context.Context, q engineeringQuery, project, head string, ids []string, versions map[string]int, facilityIDs ...[]string) (string, error) {
	rows, err := q.Query(ctx, `SELECT p.state,
 COALESCE((SELECT jsonb_agg(o.entity_id::text) FROM kickoff_publication_object o WHERE o.publication_id=p.id AND o.kind='device'),'[]'::jsonb),
 COALESCE((SELECT jsonb_agg(o.entity_id::text) FROM kickoff_publication_object o WHERE o.publication_id=p.id AND o.kind='facility'),'[]'::jsonb),
 COALESCE((SELECT jsonb_object_agg(i.table_id,i.version) FROM kickoff_publication_input i WHERE i.publication_id=p.id),'{}'::jsonb)
 FROM kickoff_publication p WHERE p.project_id=$1 AND p.model_revision=$2 AND p.state IN ('published','pending') ORDER BY p.revision DESC`, project, head)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var state string
		var rawIDs, rawFacilities, rawVersions []byte
		if err = rows.Scan(&state, &rawIDs, &rawFacilities, &rawVersions); err != nil {
			return "", err
		}
		var priorIDs []string
		var priorVersions map[string]int
		if err = json.Unmarshal(rawIDs, &priorIDs); err != nil {
			return "", fmt.Errorf("publication scope: %w", err)
		}
		var priorFacilities []string
		if err = json.Unmarshal(rawFacilities, &priorFacilities); err != nil {
			return "", fmt.Errorf("publication facilities: %w", err)
		}
		if err = json.Unmarshal(rawVersions, &priorVersions); err != nil {
			return "", fmt.Errorf("publication inputs: %w", err)
		}
		priorScope, wanted := []string{}, []string{}
		for _, id := range priorIDs {
			priorScope = append(priorScope, "device:"+id)
		}
		for _, id := range priorFacilities {
			priorScope = append(priorScope, "facility:"+id)
		}
		for _, id := range ids {
			wanted = append(wanted, "device:"+id)
		}
		for _, facilities := range facilityIDs {
			for _, id := range facilities {
				wanted = append(wanted, "facility:"+id)
			}
		}
		if kickoffScopeCovered(priorScope, wanted, priorVersions, versions) {
			return state, nil
		}
	}
	return "", rows.Err()
}
