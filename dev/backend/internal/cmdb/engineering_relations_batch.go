package cmdb

import (
	"context"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type engineeringRelationEndpoint struct {
	kind   string
	scoped bool
}
type engineeringRelationCurrent struct {
	id, from, to, definition, project, layer, ref string
	version                                       int64
}
type engineeringRelationKey struct{ definition, from, to, layer string }
type engineeringRelationSide struct{ definition, endpoint, layer string }
type engineeringRelationInsert struct {
	ID            string        `json:"id"`
	From          string        `json:"from_uid"`
	To            string        `json:"to_uid"`
	Kind          string        `json:"kind"`
	Definition    string        `json:"definition_id"`
	Project       string        `json:"project_id"`
	Layer         string        `json:"fact_layer"`
	SourceType    string        `json:"source_type"`
	Ref           string        `json:"source_ref"`
	Direction     string        `json:"direction"`
	Cardinality   string        `json:"cardinality"`
	SchemaVersion string        `json:"definition_version"`
	RevisionID    string        `json:"revision_id"`
	Payload       RelationInput `json:"payload"`
}
type engineeringRelationEnd struct {
	ID         string `json:"id"`
	Ref        string `json:"source_ref"`
	RevisionID string `json:"revision_id"`
}

func engineeringRelationLayer(layer string) string {
	if layer == "" || layer == "formal" || layer == "manual" {
		return "recorded"
	}
	return layer
}
func engineeringReplacementSide(definition, from, to string) string {
	switch definition {
	case "located_in", "cable_endpoint_a", "cable_endpoint_b":
		return from
	}
	return to
}
func engineeringCableEnd(definition string) bool {
	return definition == "cable_endpoint_a" || definition == "cable_endpoint_b"
}

// writeEngineeringRelations runs inside the caller's publication transaction, after
// all assets/scopes have been flushed. On any error the caller must roll back.
// It retains PutRelation's cross-project/layer cardinality and graph semantics;
// only this project's engineering planning edges are eligible for replacement.
func (s Service) writeEngineeringRelations(ctx context.Context, tx pgx.Tx, schema Schema, project string, relations []RelationInput) error {
	if len(relations) == 0 {
		return nil
	}
	endpointSet, definitionSet := map[string]bool{}, map[string]bool{}
	for _, in := range relations {
		if in.Project != project {
			return Reject("RELATIONSHIP_ENDPOINT_SCOPE", 422)
		}
		endpointSet[in.SourceID], endpointSet[in.TargetID] = true, true
		definitionSet[in.Definition] = true
		if engineeringCableEnd(in.Definition) {
			definitionSet["cable_endpoint_a"], definitionSet["cable_endpoint_b"] = true, true
		}
	}
	ids, definitions := make([]string, 0, len(endpointSet)), make([]string, 0, len(definitionSet))
	for id := range endpointSet {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for id := range definitionSet {
		definitions = append(definitions, id)
	}
	sort.Strings(definitions)
	rows, err := tx.Query(ctx, `SELECT e.item_uid,e.asset_type,EXISTS(SELECT 1 FROM cmdb_asset_scope sc WHERE sc.asset_id=e.item_uid AND sc.project_id=$2) FROM cmdb_entity e WHERE e.item_uid=ANY($1::text[]) ORDER BY e.item_uid FOR UPDATE OF e`, ids, project)
	if err != nil {
		return err
	}
	endpoints := make(map[string]engineeringRelationEndpoint, len(ids))
	for rows.Next() {
		var id string
		var endpoint engineeringRelationEndpoint
		if err = rows.Scan(&id, &endpoint.kind, &endpoint.scoped); err != nil {
			rows.Close()
			return err
		}
		endpoints[id] = endpoint
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	// No project or layer filter: other projects and recorded/observed facts must
	// remain visible to the same checks used by the single-relation writer.
	rows, err = tx.Query(ctx, `SELECT id::text,from_uid,to_uid,definition_id,project_id,fact_layer,source_ref,version FROM cmdb_relationship WHERE valid_to IS NULL AND definition_id=ANY($2::text[]) AND (from_uid=ANY($1::text[]) OR to_uid=ANY($1::text[])) ORDER BY id FOR UPDATE`, ids, definitions)
	if err != nil {
		return err
	}
	current := []engineeringRelationCurrent{}
	for rows.Next() {
		var r engineeringRelationCurrent
		if err = rows.Scan(&r.id, &r.from, &r.to, &r.definition, &r.project, &r.layer, &r.ref, &r.version); err != nil {
			rows.Close()
			return err
		}
		current = append(current, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	inserts, ends, err := planEngineeringRelations(schema, project, relations, endpoints, current)
	if err != nil {
		return err
	}
	if definitionSet["contains"] || definitionSet["parent"] {
		// The final graph combines existing edges across every project and fact layer
		// with the entire batch, minus only the approved planning replacements.
		var cycle, limited bool
		err = tx.QueryRow(ctx, `WITH RECURSIVE
 incoming AS (SELECT * FROM jsonb_to_recordset($1::jsonb) AS x(relationship_type text,source_asset_id text,target_asset_id text)),
 additions AS (SELECT * FROM jsonb_to_recordset($2::jsonb) AS x(definition_id text,from_uid text,to_uid text)),
 ended AS (SELECT (x->>'id')::uuid AS id FROM jsonb_array_elements($3::jsonb) x),
 edges AS (SELECT DISTINCT definition_id,from_uid,to_uid FROM (
  SELECT r.definition_id,r.from_uid,r.to_uid FROM cmdb_relationship r WHERE r.valid_to IS NULL AND r.definition_id IN ('contains','parent') AND NOT EXISTS(SELECT 1 FROM ended e WHERE e.id=r.id)
  UNION ALL SELECT definition_id,from_uid,to_uid FROM additions WHERE definition_id IN ('contains','parent')
 ) e),
 walk(root,definition,id,depth,path) AS (
  SELECT DISTINCT source_asset_id,relationship_type,target_asset_id,0,ARRAY[target_asset_id] FROM incoming WHERE relationship_type IN ('contains','parent')
  UNION ALL SELECT w.root,w.definition,e.to_uid,w.depth+1,w.path||e.to_uid FROM walk w JOIN edges e ON e.from_uid=w.id AND e.definition_id=w.definition WHERE w.depth<32 AND NOT e.to_uid=ANY(w.path)
 ) SELECT COALESCE(bool_or(id=root),false),COALESCE(bool_or(depth=32),false) FROM walk`, Encode(relations), Encode(inserts), Encode(ends)).Scan(&cycle, &limited)
		if err != nil {
			return err
		}
		if cycle {
			return Reject("RELATIONSHIP_CYCLE", 422)
		}
		if limited {
			return Reject("RELATIONSHIP_DEPTH_UNVERIFIED", 422)
		}
	}
	if len(ends) > 0 {
		_, err = tx.Exec(ctx, `WITH input AS (SELECT * FROM jsonb_to_recordset($1::jsonb) AS x(id uuid,source_ref text,revision_id uuid)),
 before_rows AS MATERIALIZED (SELECT r.id,to_jsonb(r) AS payload FROM cmdb_relationship r JOIN input i ON i.id=r.id),
 updated AS (UPDATE cmdb_relationship r SET valid_to=now(),relationship_status='historical',version=r.version+1 FROM input i,before_rows b WHERE r.id=i.id AND b.id=r.id RETURNING r.id,r.version)
 INSERT INTO cmdb_relationship_revision(id,relationship_id,version,payload,source_ref) SELECT i.revision_id,u.id,u.version,jsonb_build_object('before',b.payload,'status','historical'),i.source_ref FROM updated u JOIN input i ON i.id=u.id JOIN before_rows b ON b.id=u.id`, Encode(ends))
		if err != nil {
			return err
		}
	}
	if len(inserts) > 0 {
		_, err = tx.Exec(ctx, `WITH input AS (SELECT * FROM jsonb_to_recordset($1::jsonb) AS x(id uuid,from_uid text,to_uid text,kind text,definition_id text,project_id text,fact_layer text,source_type text,source_ref text,direction text,cardinality text,definition_version text,revision_id uuid,payload jsonb)),
 inserted AS (INSERT INTO cmdb_relationship(id,from_uid,to_uid,kind,definition_id,project_id,fact_layer,source_type,source_ref,direction,cardinality,definition_version) SELECT id,from_uid,to_uid,kind,definition_id,project_id,fact_layer,source_type,source_ref,direction,cardinality,definition_version FROM input RETURNING id)
 INSERT INTO cmdb_relationship_revision(id,relationship_id,version,payload,source_ref) SELECT i.revision_id,i.id,1,i.payload,i.source_ref FROM input i JOIN inserted n ON n.id=i.id`, Encode(inserts))
	}
	return err
}

func planEngineeringRelations(schema Schema, project string, inputs []RelationInput, endpoints map[string]engineeringRelationEndpoint, current []engineeringRelationCurrent) ([]engineeringRelationInsert, []engineeringRelationEnd, error) {
	inserts, ends := []engineeringRelationInsert{}, []engineeringRelationEnd{}
	definitions := map[string]RelationshipDefinition{}
	for _, d := range schema.Relationships {
		definitions[d.Name] = d
	}
	matches := func(kind string, types []string) bool {
		for _, t := range types {
			if schema.IsA(kind, t) {
				return true
			}
		}
		return false
	}
	replacements := map[engineeringRelationSide][]RelationInput{}
	for _, in := range inputs {
		d, ok := definitions[in.Definition]
		if !ok {
			return nil, nil, Reject("UNKNOWN_RELATIONSHIP_TYPE", 422)
		}
		if in.SourceID == in.TargetID {
			return nil, nil, Reject("RELATIONSHIP_SELF_CYCLE", 422)
		}
		if !contains(d.Sources, in.Source.Type) {
			return nil, nil, Reject("RELATIONSHIP_SOURCE_DENIED", 403)
		}
		a, aok := endpoints[in.SourceID]
		b, bok := endpoints[in.TargetID]
		if in.Project != project || !aok || !bok || !a.scoped || !b.scoped {
			return nil, nil, Reject("RELATIONSHIP_ENDPOINT_SCOPE", 422)
		}
		if !matches(a.kind, d.SourceTypes) || !matches(b.kind, d.TargetTypes) {
			return nil, nil, Reject("RELATIONSHIP_ENDPOINT_TYPE", 422)
		}
		if engineeringRelationLayer(in.Source.Layer) == "planned" && strings.HasPrefix(in.Source.Ref, "engineering:") {
			key := engineeringRelationSide{in.Definition, engineeringReplacementSide(in.Definition, in.SourceID, in.TargetID), "planned"}
			replacements[key] = append(replacements[key], in)
		}
	}
	existing := map[engineeringRelationKey]engineeringRelationCurrent{}
	fromCounts, toCounts := map[engineeringRelationSide]int{}, map[engineeringRelationSide]int{}
	occupied := map[[2]string]bool{}
	add := func(r engineeringRelationCurrent) {
		existing[engineeringRelationKey{r.definition, r.from, r.to, r.layer}] = r
		fromCounts[engineeringRelationSide{r.definition, r.from, r.layer}]++
		toCounts[engineeringRelationSide{r.definition, r.to, r.layer}]++
		if engineeringCableEnd(r.definition) {
			occupied[[2]string{r.to, r.layer}] = true
		}
	}
	for _, r := range current {
		replaceRef := ""
		if r.project == project && r.layer == "planned" && strings.HasPrefix(r.ref, "engineering:") {
			for _, in := range replacements[engineeringRelationSide{r.definition, engineeringReplacementSide(r.definition, r.from, r.to), r.layer}] {
				if in.SourceID != r.from || in.TargetID != r.to {
					replaceRef = in.Source.Ref
					break
				}
			}
		}
		if replaceRef != "" {
			ends = append(ends, engineeringRelationEnd{r.id, replaceRef, uuid.NewString()})
			continue
		}
		add(r)
	}
	for _, in := range inputs {
		d := definitions[in.Definition]
		layer := engineeringRelationLayer(in.Source.Layer)
		key := engineeringRelationKey{in.Definition, in.SourceID, in.TargetID, layer}
		if old, ok := existing[key]; ok {
			if in.ExpectedVersion != nil && *in.ExpectedVersion != old.version {
				return nil, nil, Reject("RELATIONSHIP_REVISION_CONFLICT", 409)
			}
			continue
		}
		if (d.MaxTargets > 0 && fromCounts[engineeringRelationSide{d.Name, in.SourceID, layer}] >= d.MaxTargets) || (d.MaxSources > 0 && toCounts[engineeringRelationSide{d.Name, in.TargetID, layer}] >= d.MaxSources) {
			return nil, nil, Reject("RELATIONSHIP_CARDINALITY", 422)
		}
		if engineeringCableEnd(in.Definition) && occupied[[2]string{in.TargetID, layer}] {
			return nil, nil, Reject("PORT_ALREADY_CABLED", 422)
		}
		id := in.ID
		if id == "" {
			id = uuid.NewString()
		}
		cardinality := "many-to-many"
		if d.MaxTargets == 1 && d.MaxSources == 1 {
			cardinality = "one-to-one"
		} else if d.MaxTargets == 1 {
			cardinality = "many-to-one"
		} else if d.MaxSources == 1 {
			cardinality = "one-to-many"
		}
		inserts = append(inserts, engineeringRelationInsert{ID: id, From: in.SourceID, To: in.TargetID, Kind: d.Kind, Definition: d.Name, Project: project, Layer: layer, SourceType: in.Source.Type, Ref: in.Source.Ref, Direction: d.Direction, Cardinality: cardinality, SchemaVersion: schema.Version, RevisionID: uuid.NewString(), Payload: in})
		add(engineeringRelationCurrent{id: id, from: in.SourceID, to: in.TargetID, definition: in.Definition, project: project, layer: layer, ref: in.Source.Ref, version: 1})
	}
	return inserts, ends, nil
}
