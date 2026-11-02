package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
)

func prepareEngineeringDB(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	for i, file := range []string{"027_engineering_physical.sql", "028_engineering_calculation.sql", "029_engineering_connectivity.sql", "030_engineering_patch_through.sql", "031_engineering_power.sql", "032_engineering_estimates.sql", "033_engineering_network.sql", "034_engineering_intelligence.sql", "035_engineering_reimport.sql", "036_engineering_cable_segments.sql", "037_engineering_attributes.sql", "038_engineering_analysis_artifacts.sql", "039_engineering_endpoint_identity.sql", "040_engineering_placement_inputs.sql", "041_engineering_port_connector.sql", "042_engineering_network_profile.sql"} {
		var exists bool
		if e := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migration WHERE version=$1)`, 27+i).Scan(&exists); e != nil {
			t.Fatal(e)
		}
		if exists {
			continue
		}
		raw, e := os.ReadFile("../../migrations/" + file)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(ctx, string(raw)); e != nil {
			t.Fatalf("%s: %v", file, e)
		}
	}
}
