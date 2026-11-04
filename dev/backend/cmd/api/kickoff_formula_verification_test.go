package main

import (
	"context"
	"keel.local/dev/internal/bom"
	"testing"
)

func TestKickoffNoFormulasDoesNotQueueCalculation(t *testing.T) {
	s := &Server{}
	result, err := s.kickoffVerifyWorkbook(context.Background(), "synthetic", 1, bom.Workbook{})
	if err != nil || len(result) != 0 {
		t.Fatalf("formula-free workbook: %v %v", result, err)
	}
}
