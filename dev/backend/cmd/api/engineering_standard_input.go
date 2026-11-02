package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"keel.local/dev/internal/engineering"
)

type standardInputAttachment struct{ ID, TableID, ObjectKey, SHA256 string }

func (s *Server) importStandardInput(ctx context.Context, w engineering.Workspace, q engineering.StandardInputRequest, actor string) error {
	if strings.TrimSpace(q.Reason) == "" || w.HeadRevisionID == nil {
		return errors.New("ENGINEERING_SCHEMA_INVALID")
	}
	if w.LatestJob != nil && (w.LatestJob.State == "running" || w.LatestJob.State == "pending") {
		return errors.New("ENGINEERING_JOB_ACTIVE")
	}
	if q.ExpectedRevision != w.Revision {
		return errors.New("REVISION_CONFLICT")
	}
	if e := s.prepareStandardPowerRules(ctx, []engineering.InputPreviewRequest{q.InputPreviewRequest}, actor); e != nil {
		return e
	}
	current, evidence, e := engineeringModel(ctx, s.db, *w.HeadRevisionID, s.project)
	if e != nil {
		return e
	}
	if q.TableID == "terminations" {
		current, e = s.resolveStandardInputIdentities(ctx, current, q.InputPreviewRequest)
		if e != nil {
			return e
		}
	}
	next, e := engineering.MergeStandardInput(current, q.InputPreviewRequest)
	if e != nil {
		return e
	}
	if e = s.bindStandardNetworkContext(ctx, w.ID, &next); e != nil {
		return e
	}
	var raw []byte
	var proposal engineering.EngineeringProposal
	if e = s.db.QueryRow(ctx, `SELECT proposal FROM eng_revision WHERE id=$1`, *w.HeadRevisionID).Scan(&raw); e != nil {
		return e
	}
	if e = json.Unmarshal(raw, &proposal); e != nil {
		return e
	}
	if s.objects == nil {
		return errors.New("STEP_SOURCE_STORAGE_UNAVAILABLE")
	}
	digest := sha256.Sum256([]byte(q.CSV))
	a := standardInputAttachment{ID: uuid.NewString(), TableID: q.TableID, SHA256: hex.EncodeToString(digest[:])}
	a.ObjectKey = s.project + "/engineering-inputs/" + w.ID + "/" + a.ID + ".csv"
	if _, e = s.objects.PutObject(ctx, s.bucket, a.ObjectKey, strings.NewReader(q.CSV), int64(len(q.CSV)), minio.PutObjectOptions{ContentType: "text/csv; charset=utf-8"}); e != nil {
		return errors.New("STEP_SOURCE_STORAGE_UNAVAILABLE")
	}
	_, e = s.saveEngineeringDraft(ctx, w.ID, q.ExpectedRevision, "human", actor, q.Reason, engineering.Extraction{Model: next, Evidence: evidence, TotalCells: w.TotalCells, MappedCells: w.MappedCells}, proposal, a)
	if e != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.objects.RemoveObject(cleanup, s.bucket, a.ObjectKey, minio.RemoveObjectOptions{})
	}
	return e
}

// Resolve by serial only within this project's CMDB scope. Duplicate matches
// remain blockers. Reuse existing IDs rather than create parallel identities.
func (s *Server) resolveStandardInputIdentities(ctx context.Context, m engineering.PhysicalModel, q engineering.InputPreviewRequest) (engineering.PhysicalModel, error) {
	preview := engineering.PreviewStepInput(q)
	if !preview.Valid || q.TableID != "terminations" {
		return m, errors.New("STEP_INPUT_INVALID")
	}
	serials := []string{}
	known := map[string]bool{}
	for _, v := range m.Equipment {
		if v.Serial != nil {
			known[strings.TrimSpace(*v.Serial)] = true
		}
	}
	for _, row := range preview.Rows {
		for _, col := range []int{0, 3} {
			serial := strings.TrimSpace(row[col])
			if !known[serial] {
				serials = append(serials, serial)
				known[serial] = true
			}
		}
	}

	rows, e := s.db.Query(ctx, `SELECT e.item_uid,e.serial,e.asset_type,e.name,COALESCE(e.payload->>'model','') FROM cmdb_entity e JOIN cmdb_asset_scope a ON a.asset_id=e.item_uid AND a.project_id=$1 WHERE e.serial=ANY($2::text[]) AND e.lifecycle_state<>'retired'`, s.project, serials)
	if e != nil {
		return m, e
	}
	resolved := map[string]bool{}
	ids := []string{}
	for rows.Next() {
		var id, serial, kind, label, model string
		if e = rows.Scan(&id, &serial, &kind, &label, &model); e != nil {
			break
		}
		if resolved[serial] {
			e = errors.New("STEP_SERIAL_AMBIGUOUS")
			break
		}
		resolved[serial] = true
		ids = append(ids, id)
		copy := serial
		m.Equipment = append(m.Equipment, engineering.EquipmentPlan{EntityVersion: engineering.EntityVersion{EntityID: id, Maturity: engineering.Estimated, EvidenceIDs: []string{}, Attributes: []engineering.AttributeValue{}}, EquipmentType: kind, SourceModelLabel: model, Quantity: 1, IdentityKind: "planned_instance", Label: label, Serial: &copy})
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return m, e
	}
	ids = []string{}
	selected := []string{}
	bySerial := map[string]string{}
	for _, v := range m.Equipment {
		if v.Serial != nil {
			bySerial[strings.TrimSpace(*v.Serial)] = v.EntityID
		}
	}
	for _, row := range preview.Rows {
		for _, pair := range [][2]int{{0, 2}, {3, 6}} {
			if id := bySerial[strings.TrimSpace(row[pair[0]])]; id != "" {
				ids = append(ids, id)
				selected = append(selected, id+":"+strings.TrimSpace(row[pair[1]]))
			}
		}
	}
	if len(ids) == 0 {
		return m, nil
	}
	rows, e = s.db.Query(ctx, `SELECT e.item_uid,e.payload->>'device_id',COALESCE(e.payload->>'physical_name',e.name),NULLIF(e.payload->>'mac','') FROM cmdb_entity e JOIN cmdb_asset_scope a ON a.asset_id=e.item_uid AND a.project_id=$1 WHERE e.asset_type='port' AND e.payload->>'device_id'=ANY($2::text[]) AND e.lifecycle_state<>'retired' AND (e.payload->>'device_id')||':'||COALESCE(e.payload->>'physical_name',e.name)=ANY($3::text[])`, s.project, ids, selected)
	if e != nil {
		return m, e
	}
	defer rows.Close()
	for rows.Next() {
		var id, device, name string
		var mac *string
		if e = rows.Scan(&id, &device, &name, &mac); e != nil {
			return m, e
		}
		exists := false
		for _, p := range m.Ports {
			if p.EquipmentID == device && p.Name == name {
				if p.EntityID != id {
					return m, errors.New("STEP_PORT_AMBIGUOUS")
				}
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		m.Ports = append(m.Ports, engineering.PlannedPort{EntityVersion: engineering.EntityVersion{EntityID: id, Maturity: engineering.Estimated, EvidenceIDs: []string{}, Attributes: []engineering.AttributeValue{}}, EquipmentID: device, Name: name, MAC: mac})
	}
	return m, rows.Err()
}

// Bind an explicit immutable published physical snapshot; never substitute a
// different task or infer network parameters from physical connectivity.
func (s *Server) bindStandardNetworkContext(ctx context.Context, source string, m *engineering.PhysicalModel) error {
	for i := range m.NetworkPlans {
		p := &m.NetworkPlans[i]
		if !engineering.NetworkHasContent(*p) {
			continue
		}
		if len(p.SourceIDs) == 0 {
			p.SourceIDs = []string{source}
		}
		if p.PhysicalRevisionID != "" {
			continue
		}
		options, e := s.engineeringPhysicalRevisions(ctx)
		if e != nil {
			return e
		}
		for _, option := range options {
			if option.SourceID == source && option.Target == engineering.ConnectivityTarget {
				p.PhysicalRevisionID = option.RevisionID
				break
			}
		}
	}
	return nil
}
