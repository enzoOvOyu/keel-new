package identity

import "testing"

func TestDefaultRoleBoundaries(t *testing.T) {
	roles := Roles()
	if len(roles) != 6 {
		t.Fatal("six roles required")
	}
	for _, r := range roles {
		p := Subject{Grants: []Grant{{Project: "a", Role: r.ID, Permissions: r.Permissions}}}
		if !p.Can("bom:read", "a", "", "") {
			t.Fatalf("%s cannot read", r.ID)
		}
		if p.Can("bom:read", "b", "", "") {
			t.Fatal("cross-project grant")
		}
		if r.ID == Viewer && p.Can("bom:edit", "a", "", "") {
			t.Fatal("viewer write")
		}
		if r.ID == NetworkEngineer && (p.Can("provision:execute", "a", "", "") || p.Can("cleanup:execute", "a", "", "")) {
			t.Fatal("NE inherits DE")
		}
	}
}
func TestResourceGrantAndUnion(t *testing.T) {
	p := Subject{Grants: []Grant{{Project: "a", ResourceType: "bom_import", ResourceID: "x", Permissions: []string{"bom:edit"}}, {Project: "a", Permissions: []string{"bom:read"}}}}
	if !p.Can("bom:edit", "a", "bom_import", "x") || p.Can("bom:edit", "a", "bom_import", "y") || p.Can("bom:edit", "a", "", "") {
		t.Fatal("resource boundary")
	}
	if !p.Can("bom:read", "a", "bom_import", "y") {
		t.Fatal("union")
	}
	if p.HasRole(Admin, "a", "", "") {
		t.Fatal("project grant elevated")
	}
}
func TestPasswordHash(t *testing.T) {
	h, e := HashPassword("Acceptance-only-password")
	if e != nil {
		t.Fatal(e)
	}
	if !VerifyPassword(h, "Acceptance-only-password") || VerifyPassword(h, "incorrect-password") || VerifyPassword("bad", "Acceptance-only-password") {
		t.Fatal("password verification")
	}
	if _, e = HashPassword("short"); e == nil {
		t.Fatal("short password")
	}
}
