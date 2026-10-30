package ai

import (
	"context"
	"testing"
)

func TestMockErrorsAreNotCandidates(t *testing.T) {
	for _, s := range []string{"invalid-json", "schema-failure", "timeout", "provider-error"} {
		if _, e := (Mock{s}).Suggest(context.Background(), Request{}); e == nil {
			t.Fatal(s)
		}
	}
	c, e := (Mock{"low-confidence"}).Suggest(context.Background(), Request{PromptVersion: "field/v1", SchemaVersion: "mapping/v1"})
	if e != nil || c[0].Confidence >= .5 {
		t.Fatal("low confidence lost")
	}
}
