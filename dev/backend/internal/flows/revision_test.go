package flows

import "testing"

func TestEngineRevisionMustBeFrozen(t *testing.T) {
	for _, value := range []any{nil, float64(0), float64(1.5), "1"} {
		if _, err := engineRevision(map[string]any{"revision": value}); err == nil {
			t.Fatalf("accepted invalid revision %v", value)
		}
	}
	if got, err := engineRevision(map[string]any{"revision": float64(7)}); err != nil || got != 7 {
		t.Fatalf("revision=%d err=%v", got, err)
	}
}
