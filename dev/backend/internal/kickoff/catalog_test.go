package kickoff

import "testing"

func TestInputValidation(t *testing.T) {
	cases := []struct {
		name, table string
		rows        []map[string]string
		valid       bool
	}{
		{"empty optional configuration", "rooms", []map[string]string{}, true},
		{"unknown table", "not_a_table", []map[string]string{}, false},
		{"unknown source columns are retained as an error", "rooms", []map[string]string{{"unmapped": "value"}}, false},
		{"missing numeric value remains unknown", "placements", []map[string]string{{"height_u": ""}}, true},
		{"negative dimensions are rejected", "equipment_specs", []map[string]string{{"height_u": "-1"}}, false},
		{"credentials require controlled reference", "management", []map[string]string{{"credential_ref": "plain-secret"}}, false},
		{"controlled credential reference", "management", []map[string]string{{"credential_ref": "vault://project/test"}}, true},
		{"malformed boot identity", "boot_interfaces", []map[string]string{{"mac": "invalid"}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateInput(c.table, c.rows)
			if (err == nil) != c.valid {
				t.Fatalf("validation mismatch: %v", err)
			}
		})
	}
}
