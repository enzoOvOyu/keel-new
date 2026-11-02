package main

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCablingInputFailureIsActionable(t *testing.T) {
	for _, code := range []string{"CABLE_APPROVED_RULE_REQUIRED", "CABLE_STOCK_SPEC_UNRESOLVED", "CABLE_INPUT_REQUIRED: route_edges"} {
		w := httptest.NewRecorder()
		engineeringFailure(w, errors.New(code))
		if w.Code != 422 || !strings.Contains(w.Body.String(), strings.Split(code, ":")[0]) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	engineeringFailure(w, errors.New("database unavailable"))
	if w.Code != 500 || strings.Contains(w.Body.String(), "database unavailable") {
		t.Fatal("storage failure leaked or misclassified")
	}
}
