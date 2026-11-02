package main

import (
	"context"
	"keel.local/dev/internal/identity"
	"net/http/httptest"
	"testing"
)

func TestFabricInputDigestTracksAssetVersions(t *testing.T) {
	a := fabricPublishedInput{ID: "published", ModelRevision: "model-v1", InputVersions: map[string]int{"network_roles": 2}, AssetVersions: map[string]int64{"switch-b": 2, "switch-a": 1}}
	b := fabricPublishedInput{ID: "published", ModelRevision: "model-v1", InputVersions: map[string]int{"network_roles": 2}, AssetVersions: map[string]int64{"switch-a": 1, "switch-b": 2}}
	if fabricInputDigest(a) != fabricInputDigest(b) {
		t.Fatal("map iteration changed digest")
	}
	b.AssetVersions["switch-b"] = 3
	if fabricInputDigest(a) == fabricInputDigest(b) {
		t.Fatal("asset version change was hidden")
	}
}
func TestFabricInputsRequireProjectAuthorization(t *testing.T) {
	s := &Server{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/projects/unauthorized/fabric/inputs", nil).WithContext(context.WithValue(context.Background(), principalKey{}, principal{Subject: identity.Subject{}}))
	if !s.fabricRoutes(w, r) || w.Code != 403 {
		t.Fatalf("unauthorized read: %d", w.Code)
	}
}
