package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"keel.local/dev/internal/engineering"
)

func (s *Server) standardCableRuleRunner(ctx context.Context, w engineering.Workspace, attachment standardInputAttachment, actor string) engineering.CableRuleRunner {
	type result struct {
		quantity engineering.Quantity
		run      string
		ref      engineering.VersionReference
	}
	cache := map[string]result{}
	return func(reference string, inputs []engineering.NamedValue) (engineering.Quantity, string, engineering.VersionReference, error) {
		zero := engineering.Quantity{}
		ref := engineering.VersionReference{}
		code, version, ok := strings.Cut(strings.TrimSpace(reference), "@")
		n, err := strconv.Atoi(version)
		if !ok || code == "" || err != nil || n < 1 {
			return zero, "", ref, fmt.Errorf("CABLE_RULE_REFERENCE_INVALID")
		}
		id := code
		if _, err = uuid.Parse(id); err != nil {
			id = engineering.StableID(s.project, "cabling-rule-code", code)
		}
		ref = engineering.VersionReference{ID: id, Version: n}
		key := engineering.Digest([]any{ref, inputs})
		if v, ok := cache[key]; ok {
			return v.quantity, v.run, v.ref, nil
		}
		view, err := engineeringRule(ctx, s.db, s.project, id, n)
		if errors.Is(err, pgx.ErrNoRows) {
			return zero, "", ref, fmt.Errorf("CABLE_APPROVED_RULE_REQUIRED")
		}
		if err != nil {
			return zero, "", ref, err
		}
		if view.Rule.Domain != engineering.ConnectivityDomain || view.Rule.ApprovalStatus != "approved" {
			return zero, "", ref, fmt.Errorf("CABLE_APPROVED_RULE_REQUIRED")
		}
		if err = s.engineeringRuleDependencies(ctx, view.Rule); err != nil {
			return zero, "", ref, err
		}
		run, err := engineering.RunRule(view.Rule, s.project, *w.HeadRevisionID, inputs)
		if err != nil {
			return zero, "", ref, fmt.Errorf("CABLE_RULE_INVALID: %w", err)
		}
		if run.Status != "succeeded" || len(run.Outputs) != 1 || run.Outputs[0].Name != "length" {
			return zero, "", ref, fmt.Errorf("CABLE_RULE_LENGTH_OUTPUT_REQUIRED")
		}
		value := run.Outputs[0].Value
		if value.Kind != "decimal" || value.Decimal == nil || value.Unit == nil {
			return zero, "", ref, fmt.Errorf("CABLE_RULE_LENGTH_OUTPUT_REQUIRED")
		}
		q := engineering.Quantity{Value: *value.Decimal, Unit: *value.Unit}
		run.SourceSHA256 = attachment.SHA256
		run.ModuleInstanceID = engineering.StableID(w.ID, "module", string(engineering.ConnectivityDomain))
		run.InputDigest = engineering.Digest([]any{attachment.SHA256, *w.HeadRevisionID, ref, inputs})
		claim := uuid.NewString()
		_, err = s.db.Exec(ctx, `INSERT INTO eng_calculation_run(id,project_id,source_id,revision_id,module_instance_id,provider_id,provider_version,rule_id,rule_version,source_sha256,input_digest,status,actor,requested_cells,claim_id,started_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'running',$12,'[]',$13,now())`, run.ID, s.project, w.ID, *w.HeadRevisionID, run.ModuleInstanceID, run.ProviderID, run.ProviderVersion, id, n, run.SourceSHA256, run.InputDigest, actor, claim)
		if err != nil {
			return zero, "", ref, err
		}
		if err = s.saveEngineeringCalculation(ctx, claim, run); err != nil {
			return zero, "", ref, err
		}
		cache[key] = result{q, run.ID, ref}
		return q, run.ID, ref, nil
	}
}
