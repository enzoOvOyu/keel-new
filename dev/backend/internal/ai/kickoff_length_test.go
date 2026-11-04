package ai

import "testing"

func TestKickoffConfirmedLengthUnits(t *testing.T) {
	for _, v := range []struct{ value, unit, want string }{{"25", "m", "25"}, {"25", "", ""}, {"1500mm", "m", "1.5"}, {"2 ft", "", "0.6096"}, {"25米", "", "25"}, {"unknown", "m", ""}} {
		if got := kickoffLengthMeters(v.value, v.unit); got != v.want {
			t.Errorf("%q %q got %q want %q", v.value, v.unit, got, v.want)
		}
	}
}
