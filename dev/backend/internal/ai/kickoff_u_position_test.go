package ai

import "testing"

func TestKickoffUStart(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  string
	}{
		{"U12", "12"},
		{"12U", "12"},
		{"12", "12"},
		{"  u 012  ", "12"},
		{"47u-", "47"},
		{" U47 - ", "47"},
		{"47u–", "47"},
		{"U10-U13", "10"},
		{"U13-U10", "10"},
		{"10-13U", "10"},
		{"", ""},
		{"0", ""},
		{"U0", ""},
		{"-12", ""},
		{"1.5U", ""},
		{"12-14", ""},
		{"47-", ""},
		{"-47u", ""},
		{"47u--", ""},
		{"47u-unknown", ""},
		{"U0-", ""},
		{"U10-U0", ""},
		{"U10-U13-U15", ""},
		{"B300", ""},
		{"Model-U12", ""},
		{"rack 12U", ""},
		{"999999999999999999999999U", ""},
	} {
		t.Run(tc.value, func(t *testing.T) {
			got, ok := kickoffUStart(tc.value)
			if got != tc.want || ok != (tc.want != "") {
				t.Fatalf("kickoffUStart(%q) = (%q, %v), want (%q, %v)", tc.value, got, ok, tc.want, tc.want != "")
			}
		})
	}
}

func TestKickoffOpenURangeDoesNotInferHeight(t *testing.T) {
	if start, ok := kickoffUStart("47u-"); !ok || start != "47" {
		t.Fatalf("explicit start lost: %q, %v", start, ok)
	}
	if start, height := kickoffURange("47u-"); start != "" || height != "" {
		t.Fatalf("open range inferred complete placement: start=%q height=%q", start, height)
	}
}
