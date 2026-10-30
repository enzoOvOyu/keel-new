package cmdb

import (
	"encoding/json"
	"testing"
)

func TestSchemaTypedOwnershipAndConstraints(t *testing.T) {
	s := Schema{Version: "test", Types: []AssetType{{Name: "device", Attributes: []Attribute{{Name: "model", Type: "Text"}}}, {Name: "sensor", Extends: "device", Attributes: []Attribute{{Name: "temperature", Type: "Number", Unit: "°C", Sources: []string{"bmc"}, Rules: map[string]any{"minimum": float64(-20), "maximum": float64(100)}}, {Name: "label", Type: "Text", Editable: true, Sources: []string{"manual"}, Rules: map[string]any{"max_length": float64(8)}}}}}}
	if !s.IsA("sensor", "device") {
		t.Fatal("inheritance must preserve device identity")
	}
	checks := []struct {
		name, field, source string
		value               any
		edit                bool
		want                string
	}{
		{"manual observation denied", "temperature", "manual", float64(42), true, "ATTRIBUTE_OWNER_DENIED"},
		{"negative measurement not missing", "temperature", "bmc", float64(-10), false, ""},
		{"unit bound", "temperature", "bmc", float64(101), false, "ATTRIBUTE_MAXIMUM"},
		{"manual annotation", "label", "manual", "rack-a", true, ""},
		{"unknown field", "password", "manual", "secret", true, "UNDECLARED_ATTRIBUTE"},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			problems := s.Validate("sensor", map[string]any{c.field: c.value}, c.source, c.edit)
			if c.want == "" {
				if len(problems) > 0 {
					t.Fatalf("unexpected validation: %+v", problems)
				}
				return
			}
			for _, p := range problems {
				if p.Code == c.want {
					return
				}
			}
			t.Fatalf("expected %s, got %+v", c.want, problems)
		})
	}
}
func TestVersionedSchemaExtensionPreservesCore(t *testing.T) {
	var s Schema
	if err := json.Unmarshal(builtinSchema, &s); err != nil {
		t.Fatal(err)
	}
	next, err := Extend(s, SchemaExtension{BaseVersion: s.Version, Version: "test-v2", TypeExtensions: []AssetType{{Name: "server", Attributes: []Attribute{{Name: "custom_note", Type: "Text", Editable: true, Sources: []string{"manual"}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Attributes("server")["custom_note"]; ok {
		t.Fatal("extension mutated frozen base schema")
	}
	if _, ok := next.Attributes("server")["custom_note"]; !ok {
		t.Fatal("extension missing")
	}
}
