package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"keel.local/dev/internal/bom"
	"keel.local/dev/internal/cmdb"
	"keel.local/dev/internal/engineering"
	"keel.local/dev/internal/identity"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func publishEngineeringTestTargets(t *testing.T, s *Server, source string, revision int, targets []string) {
	t.Helper()
	ctx := context.Background()
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(context.WithValue(ctx, principalKey{}, principal{Subject: identity.Subject{Username: "synthetic-network-reviewer"}}))
	r.Header.Set("Idempotency-Key", uuid.NewString())
	q := engineering.TargetAction{ExpectedRevision: revision, Targets: targets, Decision: "approve", Reason: "Synthetic referenced model approval"}
	if e := s.engineeringAction(r, source, q, "review"); e != nil {
		t.Fatal(e)
	}
	q.Decision = "publish"
	if e := s.engineeringAction(r, source, q, "publish"); e != nil {
		t.Fatal(e)
	}
	for _, target := range targets {
		tx, e := s.db.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		var event string
		var raw []byte
		e = tx.QueryRow(ctx, `SELECT o.id::text,o.payload FROM outbox_event o JOIN eng_publication p ON p.outbox_id=o.id WHERE p.source_id=$1 AND p.target=$2 AND p.state='pending' ORDER BY p.created_at DESC LIMIT 1`, source, target).Scan(&event, &raw)
		if e == nil {
			e = s.publishEngineering(ctx, tx, event, s.project, raw)
		}
		if e == nil {
			e = tx.Commit(ctx)
		} else {
			tx.Rollback(ctx)
		}
		if e != nil {
			t.Fatalf("%s: %v", target, e)
		}
	}
}
func TestEngineeringNetworkPublicationAndGoldenGate(t *testing.T) {
	dsn := os.Getenv("KEEL_ENGINEERING_TEST_DATABASE_URL")
	if !strings.Contains(dsn, "/keel_engineering_") {
		t.Skip("requires isolated engineering database")
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	prepareEngineeringDB(t, db)
	s := &Server{db: db, project: "engineering-network-" + uuid.NewString()}
	if e = (cmdb.Service{DB: db}).Initialize(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, `INSERT INTO project(id,name,code) VALUES($1,'Synthetic network planning',$1)`, s.project); e != nil {
		t.Fatal(e)
	}
	source := func() string {
		batch := uuid.NewString()
		if _, e = db.Exec(ctx, `INSERT INTO bom_import_batch(id,project_id,source_document_id,object_key,filename,content_sha256,status,workbook) VALUES($1,$2,$3,$4,'synthetic.xlsx','synthetic-hash','MAPPING',$5)`, batch, s.project, uuid.NewString(), s.project+"/"+batch, jsonBytes(bom.Workbook{Sheets: []bom.Sheet{}})); e != nil {
			t.Fatal(e)
		}
		id, e := s.attachEngineering(ctx, batch, "synthetic")
		if e != nil {
			t.Fatal(e)
		}
		return id
	}
	physicalSource, networkSource := source(), source()
	version := func() engineering.EntityVersion {
		return engineering.EntityVersion{EntityID: uuid.NewString(), Maturity: engineering.Estimated}
	}
	a, z, ap, zp, connection := version(), version(), version(), version(), version()
	m := engineering.EmptyPhysical(s.project)
	m.Equipment = []engineering.EquipmentPlan{{EntityVersion: a, Label: "Synthetic A", Quantity: 1, EquipmentType: "switch", IdentityKind: "planned_instance"}, {EntityVersion: z, Label: "Synthetic Z", Quantity: 1, EquipmentType: "switch", IdentityKind: "planned_instance"}}
	m.Ports = []engineering.PlannedPort{{EntityVersion: ap, EquipmentID: a.EntityID, Name: "p1"}, {EntityVersion: zp, EquipmentID: z.EntityID, Name: "p2"}}
	m.Connections = []engineering.PlannedConnection{{EntityVersion: connection, APortID: ap.EntityID, ZPortID: zp.EntityID, PatchPortIDs: []string{}}}
	proposal := engineering.EngineeringProposal{Summary: "Synthetic independent network", Regions: []engineering.RegionMapping{}, Modules: []engineering.ModuleDetection{}, Questions: []string{}}
	physicalRev, e := s.saveEngineeringDraft(ctx, physicalSource, 1, "human", "synthetic-human", "Explicit physical fixture", engineering.Extraction{Model: m}, proposal)
	if e != nil {
		t.Fatal(e)
	}
	plan := engineering.EmptyNetwork(networkSource)
	plan.EntityVersion = version()
	plan.PhysicalRevisionID = physicalRev
	fabric, plane, ai, zi, aa, za, ipA, ipZ, bgp := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	astr, zstr := "192.0.2.0", "192.0.2.1"
	aASN, zASN := int64(65001), int64(65002)
	prefix, roce, vlan := 31, 2, 100
	enabled := true
	plan.Fabrics = []engineering.NetworkFabric{{ID: fabric, Name: "Synthetic fabric", Kind: "ethernet"}}
	plan.Planes = []engineering.NetworkPlane{{ID: plane, FabricID: fabric, Name: "Synthetic plane"}}
	plan.DeviceRoles = []engineering.DeviceRole{{EquipmentID: a.EntityID, PlaneID: plane, Role: "leaf"}, {EquipmentID: z.EntityID, PlaneID: plane, Role: "spine"}}
	plan.InterfaceMappings = []engineering.InterfaceMapping{{ID: ai, PhysicalPortID: ap.EntityID, PlaneID: plane, LogicalName: "eth1"}, {ID: zi, PhysicalPortID: zp.EntityID, PlaneID: plane, LogicalName: "eth2"}}
	plan.Allocations = []engineering.NetworkAllocation{{ID: aa, Kind: "asn", EntityID: a.EntityID, ASN: &aASN}, {ID: za, Kind: "asn", EntityID: z.EntityID, ASN: &zASN}, {ID: ipA, Kind: "ip", EntityID: a.EntityID, InterfaceID: &ai, Address: &astr, PrefixLength: &prefix}, {ID: ipZ, Kind: "ip", EntityID: z.EntityID, InterfaceID: &zi, Address: &zstr, PrefixLength: &prefix}}
	plan.BGP = []engineering.BGPPlan{{ID: bgp, AInterfaceID: ai, ZInterfaceID: zi, LocalASNAllocationID: aa, PeerASNAllocationID: za, AFI: "ipv4", SAFI: "unicast"}}
	plan.BFD = []engineering.BFDPlan{{SessionID: bgp, MinTXMilliseconds: 300, MinRXMilliseconds: 300, Multiplier: 3}}
	plan.QoS = []engineering.QoSPlan{{ID: uuid.NewString(), InterfaceIDs: []string{ai, zi}, RoCEVersion: &roce, LosslessPriorities: []int{3}, ECNEnabled: &enabled, PFCEnabled: &enabled}}
	plan.AdaptiveRouting = []engineering.AdaptiveRoutingPlan{{FabricID: fabric, Enabled: false}}
	plan.EVPNVXLAN = []engineering.EVPNVXLANPlan{{ID: uuid.NewString(), FabricID: fabric, VNI: 1000, VLAN: &vlan, VTEPAllocationIDs: []string{ipA}, RouteDistinguisher: "65001:100", ImportTargets: []string{"65001:100"}, ExportTargets: []string{"65001:100"}}}
	if e = s.editEngineeringNetwork(ctx, networkSource, engineering.NetworkEdit{ExpectedRevision: 1, Plan: plan, Reason: "Explicit synthetic logical facts"}, "synthetic-human"); e != nil {
		t.Fatal(e)
	}
	w, e := s.engineeringWorkspace(ctx, networkSource)
	if e != nil {
		t.Fatal(e)
	}
	blocked := false
	for _, i := range w.Issues {
		blocked = blocked || i.Code == "NETWORK_PHYSICAL_NOT_PUBLISHED"
	}
	if !blocked {
		t.Fatal("unpublished physical context accepted")
	}
	if _, e = s.engineeringGoldenInput(ctx, networkSource); e == nil {
		t.Fatal("draft network consumed by Golden")
	}
	publishEngineeringTestTargets(t, s, physicalSource, 2, []string{engineering.EquipmentTarget, engineering.ConnectivityTarget})
	if e = s.editEngineeringNetwork(ctx, networkSource, engineering.NetworkEdit{ExpectedRevision: 2, Plan: plan, Reason: "Revalidate published physical context"}, "synthetic-human"); e != nil {
		t.Fatal(e)
	}
	w, e = s.engineeringWorkspace(ctx, networkSource)
	if e != nil {
		t.Fatal(e)
	}
	for _, i := range w.Issues {
		if i.PublishTarget == engineering.NetworkTarget && i.Severity == "BLOCK" {
			t.Fatal(i)
		}
	}
	var before, after int
	if e = db.QueryRow(ctx, `SELECT count(*) FROM cmdb_asset_scope WHERE project_id=$1`, s.project).Scan(&before); e != nil {
		t.Fatal(e)
	}
	publishEngineeringTestTargets(t, s, networkSource, 3, []string{engineering.NetworkTarget})
	input, e := s.engineeringGoldenInput(ctx, networkSource)
	if e != nil {
		t.Fatal(e)
	}
	if input.PhysicalRevisionID != physicalRev || len(input.Plan.Allocations) != 4 || input.Plan.Allocations[2].Address == nil || len(input.Plan.BGP) != 1 || len(input.Plan.EVPNVXLAN) != 1 || len(input.Plan.QoS[0].InterfaceIDs) != 2 {
		t.Fatalf("typed network roundtrip lost facts: %+v", input.Plan)
	}
	if e = db.QueryRow(ctx, `SELECT count(*) FROM cmdb_asset_scope WHERE project_id=$1`, s.project).Scan(&after); e != nil {
		t.Fatal(e)
	}
	if after != before {
		t.Fatal("network plan fabricated physical assets")
	}
	_, x, p, e := s.engineeringEditContext(ctx, physicalSource)
	if e != nil {
		t.Fatal(e)
	}
	x.Model.Ports[0].Name = "changed-planned-port"
	if _, e = s.saveEngineeringDraft(ctx, physicalSource, 2, "human", "synthetic-human", "Change physical input", x, p); e != nil {
		t.Fatal(e)
	}
	if _, e = s.engineeringGoldenInput(ctx, networkSource); e == nil || e.Error() != "NETWORK_PHYSICAL_REVISION_STALE" {
		t.Fatalf("stale physical revision consumed by Golden: %v", e)
	}
}
