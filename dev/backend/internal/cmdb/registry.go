package cmdb

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	"net"
	"regexp"
	"sort"
	"strings"
	"time"
)

//go:embed schemas/core-v1.json
var builtinSchema []byte

type Attribute struct {
	Name       string         `json:"name"`
	Label      string         `json:"label"`
	Type       string         `json:"type"`
	Unit       string         `json:"unit,omitempty"`
	Required   bool           `json:"required"`
	Deprecated bool           `json:"deprecated"`
	Unique     bool           `json:"unique"`
	Editable   bool           `json:"editable"`
	Sources    []string       `json:"sources"`
	Models     []string       `json:"models"`
	Group      string         `json:"group"`
	Enum       []string       `json:"enum,omitempty"`
	Rules      map[string]any `json:"rules"`
}
type AssetType struct {
	Name            string         `json:"name"`
	Label           string         `json:"label"`
	Extends         string         `json:"extends"`
	Attributes      []Attribute    `json:"attributes"`
	AllowExtensions bool           `json:"allow_extensions"`
	Display         map[string]any `json:"display"`
}
type RelationshipDefinition struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Kind        string   `json:"kind"`
	SourceTypes []string `json:"source_types"`
	TargetTypes []string `json:"target_types"`
	Direction   string   `json:"direction"`
	MaxTargets  int      `json:"max_targets"`
	MaxSources  int      `json:"max_sources"`
	Sources     []string `json:"sources"`
}
type Schema struct {
	Version       string                   `json:"version"`
	Types         []AssetType              `json:"types"`
	Relationships []RelationshipDefinition `json:"relationships"`
}
type Service struct{ DB *pgxpool.Pool }
type Problem struct {
	Code    string `json:"code"`
	AssetID string `json:"asset_id"`
	Field   string `json:"field"`
	Reason  string `json:"reason"`
}
type Error struct {
	Status   int
	Code     string
	Problems []Problem
}

func (e *Error) Error() string             { return e.Code }
func Reject(code string, status int) error { return &Error{Status: status, Code: code} }
func Encode(v any) []byte                  { b, _ := json.Marshal(v); return b }
func Hash(v any) string                    { h := sha256.Sum256(Encode(v)); return hex.EncodeToString(h[:]) }
func (s Schema) Type(kind string) (AssetType, bool) {
	for _, t := range s.Types {
		if t.Name == kind {
			return t, true
		}
	}
	return AssetType{}, false
}
func (s Schema) IsA(kind, parent string) bool {
	if parent == "*" {
		return true
	}
	seen := map[string]bool{}
	for kind != "" && !seen[kind] {
		if kind == parent {
			return true
		}
		seen[kind] = true
		t, ok := s.Type(kind)
		if !ok {
			return false
		}
		kind = t.Extends
	}
	return false
}
func (s Schema) Definition(name string) (RelationshipDefinition, bool) {
	for _, d := range s.Relationships {
		if d.Name == name {
			return d, true
		}
	}
	return RelationshipDefinition{}, false
}
func (s Schema) Attributes(kind string) map[string]Attribute {
	out := map[string]Attribute{}
	seen := map[string]bool{}
	var load func(string)
	load = func(k string) {
		if seen[k] {
			return
		}
		seen[k] = true
		if t, ok := s.Type(k); ok {
			load(t.Extends)
			for _, a := range t.Attributes {
				out[a.Name] = a
			}
		}
	}
	load(kind)
	return out
}
func (s Service) Active(ctx context.Context) (Schema, error) {
	var raw []byte
	var schema Schema
	e := s.DB.QueryRow(ctx, "SELECT document FROM cmdb_schema_version WHERE status='active'").Scan(&raw)
	if e == nil {
		e = json.Unmarshal(raw, &schema)
	}
	return schema, e
}
func (s Service) Initialize(ctx context.Context) error {
	var exists bool
	if e := s.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM cmdb_schema_version)").Scan(&exists); e != nil {
		return e
	}
	if !exists {
		var schema Schema
		if e := json.Unmarshal(builtinSchema, &schema); e != nil {
			return e
		}
		_, e := s.Install(ctx, schema, "", "bootstrap")
		if e != nil {
			return e
		}
	}
	_, e := s.DB.Exec(ctx, "UPDATE outbox_event e SET published_at=NULL WHERE event_type='bom.committed' AND NOT EXISTS(SELECT 1 FROM cmdb_ingest_event i WHERE i.event_id=e.id::text)")
	return e
}
func CheckSchema(schema Schema) []Problem {
	out := []Problem{}
	seen := map[string]bool{}
	validName := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	if schema.Version == "" {
		out = append(out, Problem{Code: "SCHEMA_VERSION_REQUIRED"})
	}
	for _, t := range schema.Types {
		if !validName.MatchString(t.Name) || seen[t.Name] {
			out = append(out, Problem{Code: "INVALID_ASSET_TYPE", Field: t.Name})
		}
		seen[t.Name] = true
		fields := map[string]bool{}
		for _, a := range t.Attributes {
			if fields[a.Name] || !validName.MatchString(a.Name) {
				out = append(out, Problem{Code: "INVALID_ATTRIBUTE", Field: a.Name})
			}
			fields[a.Name] = true
			switch a.Type {
			case "Text", "Number", "Boolean", "Enum", "IP", "MAC", "DateTime", "Reference", "Array", "Object":
			default:
				out = append(out, Problem{Code: "INVALID_ATTRIBUTE_TYPE", Field: a.Name})
			}
		}
	}
	for _, t := range schema.Types {
		if t.Extends != "" && !seen[t.Extends] {
			out = append(out, Problem{Code: "UNKNOWN_PARENT_TYPE", Field: t.Name})
		}
		visited := map[string]bool{}
		for k := t.Name; k != ""; {
			if visited[k] {
				out = append(out, Problem{Code: "SCHEMA_INHERITANCE_CYCLE", Field: t.Name})
				break
			}
			visited[k] = true
			p, ok := schema.Type(k)
			if !ok {
				break
			}
			k = p.Extends
		}
	}
	rels := map[string]bool{}
	for _, r := range schema.Relationships {
		if rels[r.Name] || !validName.MatchString(r.Name) {
			out = append(out, Problem{Code: "INVALID_RELATIONSHIP", Field: r.Name})
		}
		rels[r.Name] = true
		for _, kind := range append(append([]string{}, r.SourceTypes...), r.TargetTypes...) {
			if kind != "*" && !seen[kind] {
				out = append(out, Problem{Code: "UNKNOWN_RELATIONSHIP_TYPE", Field: r.Name})
			}
		}
	}
	return out
}
func (s Service) Compatibility(ctx context.Context, next Schema) ([]Problem, error) {
	issues := CheckSchema(next)
	old, e := s.Active(ctx)
	if e == pgx.ErrNoRows {
		return issues, nil
	}
	if e != nil {
		return nil, e
	}
	for _, t := range old.Types {
		n, ok := next.Type(t.Name)
		if !ok {
			issues = append(issues, Problem{Code: "ASSET_TYPE_REMOVAL_REQUIRES_MIGRATION", Field: t.Name})
			continue
		}
		for key, a := range old.Attributes(t.Name) {
			b, ok := next.Attributes(n.Name)[key]
			if !ok || a.Type != b.Type || a.Unit != b.Unit {
				issues = append(issues, Problem{Code: "INCOMPATIBLE_ATTRIBUTE", Field: t.Name + "." + key})
			}
		}
	}
	for _, r := range old.Relationships {
		n, ok := next.Definition(r.Name)
		if !ok || Hash(n) != Hash(r) {
			issues = append(issues, Problem{Code: "RELATIONSHIP_CHANGE_REQUIRES_MIGRATION", Field: r.Name})
		}
	}
	rows, e := s.DB.Query(ctx, "SELECT item_uid,asset_type,payload FROM cmdb_entity")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	uniqueValues := map[string]string{}
	for rows.Next() {
		var id, kind string
		var raw []byte
		if e = rows.Scan(&id, &kind, &raw); e != nil {
			return nil, e
		}
		var attrs map[string]any
		json.Unmarshal(raw, &attrs)
		for field, a := range next.Attributes(kind) {
			if a.Unique && attrs[field] != nil && stringValue(attrs[field]) != "" {
				key := kind + ":" + field + ":" + Hash(attrs[field])
				if oldID := uniqueValues[key]; oldID != "" {
					issues = append(issues, Problem{Code: "UNIQUE_ATTRIBUTE_CONFLICT", AssetID: id, Field: field, Reason: "Conflicts with " + oldID})
				}
				uniqueValues[key] = id
			}
		}
		for _, p := range next.Validate(kind, attrs, "", false) {
			p.AssetID = id
			issues = append(issues, p)
		}
	}
	return issues, rows.Err()
}
func (s Service) Install(ctx context.Context, schema Schema, expected, actor string) (map[string]any, error) {
	issues, e := s.Compatibility(ctx, schema)
	if e != nil {
		return nil, e
	}
	if len(issues) > 0 {
		return nil, &Error{422, "SCHEMA_INCOMPATIBLE", issues}
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	_, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(1123199014)")
	if e != nil {
		return nil, e
	}
	var active string
	e = tx.QueryRow(ctx, "SELECT id FROM cmdb_schema_version WHERE status='active'").Scan(&active)
	if e != nil && e != pgx.ErrNoRows {
		return nil, e
	}
	if active != expected {
		return nil, Reject("SCHEMA_VERSION_CONFLICT", 409)
	}
	var exists bool
	tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM cmdb_schema_version WHERE id=$1)", schema.Version).Scan(&exists)
	if exists {
		return nil, Reject("SCHEMA_VERSION_IMMUTABLE", 409)
	}
	if _, e = tx.Exec(ctx, "UPDATE cmdb_schema_version SET status='retired' WHERE status='active'"); e != nil {
		return nil, e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO cmdb_schema_version(id,document,content_hash,status,created_by) VALUES($1,$2,$3,'active',$4)", schema.Version, Encode(schema), Hash(schema), actor); e != nil {
		return nil, e
	}
	for _, t := range schema.Types {
		if _, e = tx.Exec(ctx, "INSERT INTO cmdb_schema_asset_type VALUES($1,$2,$3)", schema.Version, t.Name, Encode(t)); e != nil {
			return nil, e
		}
		for _, a := range t.Attributes {
			if _, e = tx.Exec(ctx, "INSERT INTO cmdb_schema_attribute VALUES($1,$2,$3,$4)", schema.Version, t.Name, a.Name, Encode(a)); e != nil {
				return nil, e
			}
		}
	}
	for _, r := range schema.Relationships {
		if _, e = tx.Exec(ctx, "INSERT INTO cmdb_relationship_definition VALUES($1,$2,$3)", schema.Version, r.Name, Encode(r)); e != nil {
			return nil, e
		}
	}
	// Rebuild uniqueness slots when constraints change, including existing assets.
	if _, e = tx.Exec(ctx, "DELETE FROM cmdb_unique_attribute"); e != nil {
		return nil, e
	}
	existingRows, e := tx.Query(ctx, "SELECT item_uid,asset_type,payload FROM cmdb_entity")
	if e != nil {
		return nil, e
	}
	slots := &pgx.Batch{}
	for existingRows.Next() {
		var id, kind string
		var raw []byte
		if e = existingRows.Scan(&id, &kind, &raw); e != nil {
			existingRows.Close()
			return nil, e
		}
		var attrs map[string]any
		if e = json.Unmarshal(raw, &attrs); e != nil {
			existingRows.Close()
			return nil, e
		}
		for field, a := range schema.Attributes(kind) {
			if a.Unique && attrs[field] != nil && stringValue(attrs[field]) != "" {
				slots.Queue("INSERT INTO cmdb_unique_attribute(asset_type,field_name,value_key,asset_id) VALUES($1,$2,$3,$4)", kind, field, Hash(attrs[field]), id)
			}
		}
	}
	e = existingRows.Err()
	existingRows.Close()
	if e != nil {
		return nil, e
	}
	if slots.Len() > 0 {
		result := tx.SendBatch(ctx, slots)
		if e = result.Close(); e != nil {
			return nil, Reject("UNIQUE_ATTRIBUTE_CONFLICT", 422)
		}
	}
	if _, e = tx.Exec(ctx, "INSERT INTO audit_event(id,project_id,actor,action,resource_id,details) VALUES(gen_random_uuid(),'system',$1,'cmdb.schema.installed',$2,$3)", actor, schema.Version, Encode(map[string]any{"before": expected, "after": schema.Version, "compatibility_checked": true})); e != nil {
		return nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return map[string]any{"version": schema.Version, "status": "active", "historical_assets_validated": expected != ""}, nil
}
func (s Schema) Validate(kind string, values map[string]any, source string, edit bool) []Problem {
	out := []Problem{}
	t, ok := s.Type(kind)
	if !ok {
		return []Problem{{Code: "UNKNOWN_ASSET_TYPE", Field: kind}}
	}
	attrs := s.Attributes(kind)
	for key, value := range values {
		a, known := attrs[key]
		if !known {
			if !t.AllowExtensions {
				out = append(out, Problem{Code: "UNDECLARED_ATTRIBUTE", Field: key})
			}
			continue
		}
		if source != "" && edit && (a.Deprecated || !a.Editable || !contains(a.Sources, source)) {
			out = append(out, Problem{Code: "ATTRIBUTE_OWNER_DENIED", Field: key})
			continue
		}
		if value == nil {
			continue
		}
		valid := true
		switch a.Type {
		case "Text", "Reference":
			_, valid = value.(string)
		case "Number":
			_, valid = value.(float64)
		case "Boolean":
			_, valid = value.(bool)
		case "Enum":
			v, ok := value.(string)
			valid = ok && contains(a.Enum, v)
		case "IP":
			v, ok := value.(string)
			valid = ok && net.ParseIP(v) != nil
		case "MAC":
			v, ok := value.(string)
			_, e := net.ParseMAC(v)
			valid = ok && e == nil
		case "DateTime":
			v, ok := value.(string)
			_, e := time.Parse(time.RFC3339, v)
			valid = ok && e == nil
		case "Array":
			_, valid = value.([]any)
		case "Object":
			_, valid = value.(map[string]any)
		}
		if !valid {
			out = append(out, Problem{Code: "ATTRIBUTE_TYPE_MISMATCH", Field: key})
		}
		if len(a.Models) > 0 && !contains(a.Models, stringValue(values["model"])) {
			out = append(out, Problem{Code: "ATTRIBUTE_MODEL_NOT_APPLICABLE", Field: key})
		}
		if v, ok := value.(float64); ok && a.Type == "Number" {
			if min, ok := a.Rules["minimum"].(float64); ok && v < min {
				out = append(out, Problem{Code: "ATTRIBUTE_MINIMUM", Field: key})
			}
			if max, ok := a.Rules["maximum"].(float64); ok && v > max {
				out = append(out, Problem{Code: "ATTRIBUTE_MAXIMUM", Field: key})
			}
			if integer, ok := a.Rules["integer"].(bool); ok && integer && math.Trunc(v) != v {
				out = append(out, Problem{Code: "ATTRIBUTE_INTEGER", Field: key})
			}
		}
		if v, ok := value.(string); ok {
			if max, ok := a.Rules["max_length"].(float64); ok && len(v) > int(max) {
				out = append(out, Problem{Code: "ATTRIBUTE_LENGTH", Field: key})
			}
			if pattern, ok := a.Rules["pattern"].(string); ok {
				r, e := regexp.Compile(pattern)
				if e != nil || !r.MatchString(v) {
					out = append(out, Problem{Code: "ATTRIBUTE_PATTERN", Field: key})
				}
			}
		}
	}
	if !edit {
		for key, a := range attrs {
			if a.Required && (values[key] == nil || values[key] == "") {
				out = append(out, Problem{Code: "REQUIRED_ATTRIBUTE", Field: key})
			}
		}
	}
	return out
}
func contains(items []string, v string) bool {
	for _, s := range items {
		if s == v {
			return true
		}
	}
	return false
}
func stringValue(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}
func canonicalKind(kind string) string {
	switch kind {
	case "optic_module":
		return "optic"
	case "physical_port":
		return "port"
	}
	return strings.ToLower(kind)
}
func sortedKeys(m map[string]any) []string {
	k := []string{}
	for s := range m {
		k = append(k, s)
	}
	sort.Strings(k)
	return k
}

type SchemaExtension struct {
	BaseVersion      string                   `json:"base_version"`
	Version          string                   `json:"version"`
	TypeExtensions   []AssetType              `json:"type_extensions"`
	NewTypes         []AssetType              `json:"new_types"`
	NewRelationships []RelationshipDefinition `json:"new_relationships"`
}

func Extend(base Schema, patch SchemaExtension) (Schema, error) {
	if base.Version != patch.BaseVersion {
		return Schema{}, Reject("SCHEMA_BASE_VERSION_CONFLICT", 409)
	}
	var next Schema
	json.Unmarshal(Encode(base), &next)
	next.Version = patch.Version
	for _, extension := range patch.TypeExtensions {
		found := false
		for i, t := range next.Types {
			if t.Name == extension.Name {
				found = true
				seen := next.Attributes(t.Name)
				for _, a := range extension.Attributes {
					if _, exists := seen[a.Name]; exists {
						return Schema{}, Reject("SCHEMA_EXTENSION_ATTRIBUTE_EXISTS", 409)
					}
					next.Types[i].Attributes = append(next.Types[i].Attributes, a)
				}
			}
		}
		if !found {
			return Schema{}, Reject("SCHEMA_EXTENSION_TYPE_NOT_FOUND", 422)
		}
	}
	next.Types = append(next.Types, patch.NewTypes...)
	next.Relationships = append(next.Relationships, patch.NewRelationships...)
	return next, nil
}
