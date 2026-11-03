package main

import (
	"context"
	"keel.local/dev/internal/cmdb"
	fab "keel.local/dev/internal/fabric"
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

func TestFabricMissingIntentAndCapabilityRemainBlocked(t *testing.T) {
	state := fab.NewState("project")
	input := fabricPublishedInput{ID: "publication", Project: "project", ModelRevision: "revision", Assets: []cmdb.VisualAsset{{ID: "switch", Type: "switch"}}}
	state.Input = &input.ID
	state.Blueprints = []fab.Blueprint{{ID: "bp", Input: input.ID, Status: "COMMITTED"}}
	state.Profiles = fabricIntentProfiles()
	server := &Server{}
	if _, err := server.fabricCompile(&state, &input, fab.Command{Domain: "SWITCH", IDs: []string{"switch"}}); err != nil {
		t.Fatal(err)
	}
	artifact := state.Artifacts[0]
	if state.Builds[0].Status != "INVALID" || artifact.Commands != "" || artifact.Validation != "BLOCKED" || len(artifact.Missing) == 0 {
		t.Fatal("missing NetworkPlan/native renderer fabricated executable configuration")
	}
	if artifact.Parameters.PFC != nil || artifact.Parameters.ASN != nil {
		t.Fatal("missing baseline values became defaults")
	}
	if _, err := server.fabricValidate(&state, &input); err != nil {
		t.Fatal(err)
	}
	report := state.Validations[0]
	if report.Status != "BLOCKED" || report.Blocked == 0 || report.Unsupported == 0 {
		t.Fatal("required missing evidence incorrectly passed")
	}
}
func TestFabricWorkspaceRejectsUnauthorizedWrites(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/v1/projects/foreign/fabric/commands", nil).WithContext(context.WithValue(context.Background(), principalKey{}, principal{Subject: identity.Subject{}}))
	w := httptest.NewRecorder()
	if !(&Server{}).fabricStudioRoutes(w, r) || w.Code != 403 {
		t.Fatalf("unauthorized Fabric command: %d", w.Code)
	}
}
