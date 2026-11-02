package flows

import (
	"net/url"
	"reflect"
	"testing"
)

func TestCatalogLiveStateAndSourceFilters(t *testing.T) {
	values := []Flow{{ID: "1", Name: "GPU preflight", Namespace: "ns-a", TargetType: "server", Draft: true, Engine: map[string]any{"revision": 2}}, {ID: "2", Name: "Network", Namespace: "ns-b", TargetType: "switch", Disabled: true, Engine: map[string]any{"revision": 1}}, {ID: "3", Name: "Missing engine", Namespace: "ns-a", TargetType: "server", Engine: map[string]any{}}}
	filtered, names := catalogFilter(values, url.Values{"q": {"gpu"}, "state": {"DRAFT"}, "target_type": {"server"}})
	if len(filtered) != 1 || filtered[0].ID != "1" || !reflect.DeepEqual(names, []string{"ns-a", "ns-b"}) {
		t.Fatalf("unexpected catalog %v %v", filtered, names)
	}
	if flowStatus(values[2]) != "unknown" {
		t.Fatal("missing engine state became enabled")
	}
}
func TestWorkspacePaginationAndTimeFilters(t *testing.T) {
	for _, q := range []url.Values{{"limit": {"-1"}}, {"limit": {"bad"}}, {"limit": {"101"}}} {
		if _, e := queryInt(q, "limit", 25, 100); e == nil {
			t.Fatal("invalid limit accepted", q)
		}
	}
	if _, _, e := queryTimes(url.Values{"since": {"2026-11-03T00:00:00Z"}, "until": {"2026-11-02T00:00:00Z"}}); e == nil {
		t.Fatal("reversed time window accepted")
	}
	if _, _, e := queryTimes(url.Values{"since": {"invalid"}}); e == nil {
		t.Fatal("invalid time accepted")
	}
}
func TestExecutionVariableSecretFieldsAreMasked(t *testing.T) {
	value := map[string]any{"normal": float64(0), "nested": []any{map[string]any{"api_key": "private", "value": "sample"}}, "password": "private", "ticket": "ticket-value"}
	got := scrubVariables(value).(map[string]any)
	if got["password"] != "[REDACTED]" || got["ticket"] != "[REDACTED]" || got["normal"] != float64(0) || got["nested"].([]any)[0].(map[string]any)["api_key"] != "[REDACTED]" {
		t.Fatal("secret or zero value handling is incorrect")
	}
	if value["password"] != "private" {
		t.Fatal("scrubber mutated domain input")
	}
}

func TestDurationSummaryUsesActualSamplesAndDoesNotFillMissing(t *testing.T) {
	empty := durationSummary(nil, 3)
	if empty.Mean != nil || empty.P95 != nil || empty.MissingCount != 3 {
		t.Fatal("missing duration synthesized", empty)
	}
	actual := durationSummary([]float64{9, 2, 4}, 4)
	if actual.SampleCount != 3 || actual.MissingCount != 1 || *actual.Mean != 5 || *actual.P50 != 4 || *actual.P95 != 9 {
		t.Fatal("sample percentiles incorrect", actual)
	}
}
