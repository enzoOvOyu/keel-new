package main

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"keel.local/dev/internal/engineering"
	"keel.local/dev/internal/kickoff"
	"net/http"
)

// Replays an immutable, same-project historical source and its typed candidates.
// This copies evidence, not an old workflow state, and never invokes a provider.
func (s *Server) kickoffHistory(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SourceID         string `json:"sourceId"`
		ExpectedRevision int    `json:"expectedRevision"`
	}
	if !engineeringDecode(w, r, &body) {
		return
	}
	ctx := r.Context()
	tx, e := s.db.Begin(ctx)
	if e != nil {
		fail(w, 500, "KICKOFF_HISTORY_UNAVAILABLE")
		return
	}
	defer tx.Rollback(ctx)
	if e = kickoffLock(ctx, tx, s.project, body.ExpectedRevision); e != nil {
		kickoffFailure(w, e)
		return
	}
	var current *string
	if e = tx.QueryRow(ctx, `SELECT source_id::text FROM kickoff_workspace WHERE project_id=$1`, s.project).Scan(&current); e != nil {
		kickoffFailure(w, e)
		return
	}
	if current != nil {
		fail(w, 409, "历史复用只用于尚未开始的开局工作区")
		return
	}
	var revision, name, key, digest string
	var raw []byte
	e = tx.QueryRow(ctx, `SELECT s.head_revision_id::text,b.filename,b.object_key,b.content_sha256,r.proposal FROM eng_source s JOIN bom_import_batch b ON b.id=s.import_id JOIN eng_revision r ON r.id=s.head_revision_id WHERE s.id::text=$1 AND s.project_id=$2`, body.SourceID, s.project).Scan(&revision, &name, &key, &digest, &raw)
	if e != nil {
		fail(w, 404, "KICKOFF_HISTORY_NOT_FOUND")
		return
	}
	model, evidence, e := engineeringModel(ctx, tx, revision, s.project)
	if e != nil {
		kickoffFailure(w, e)
		return
	}
	var proposal engineering.EngineeringProposal
	if e = json.Unmarshal(raw, &proposal); e != nil {
		kickoffFailure(w, e)
		return
	}
	// Reuse confirmed source-location candidates by exact SN. Unknown dimensions
	// remain unknown; these candidates do not become published placements.
	history := engineering.Workspace{ID: body.SourceID, ProjectID: s.project, HeadRevisionID: &revision}
	values, e := s.modelingInputs(ctx, history, true)
	if e != nil {
		kickoffFailure(w, e)
		return
	}
	inputs := []engineering.ModelingInput{}
	for _, v := range values {
		inputs = append(inputs, v.ModelingInput)
	}
	refs := engineering.ResolveEndpointPlacements(model, inputs)
	serials := map[string]string{}
	for _, device := range model.Equipment {
		if device.Serial != nil {
			serials[*device.Serial] = device.EntityID
		}
	}
	facilities := map[string]bool{}
	for _, f := range model.Facilities {
		facilities[f.EntityID] = true
	}
	placed := map[string]bool{}
	for _, p := range model.Placements {
		placed[p.EquipmentID] = true
	}
	for _, ref := range refs {
		device := serials[ref.Serial]
		if device == "" || placed[device] || ref.State != "candidate" || ref.RackCode == "" {
			continue
		}
		rack := engineering.StableID(s.project, "rack", ref.RackCode)
		if !facilities[rack] {
			model.Facilities = append(model.Facilities, engineering.FacilityModel{EntityVersion: engineering.EntityVersion{EntityID: rack, Maturity: engineering.Estimated}, Kind: "rack", Code: ref.RackCode, Label: ref.RackCode})
			facilities[rack] = true
		}
		model.Placements = append(model.Placements, engineering.RackPlacement{EntityVersion: engineering.EntityVersion{EntityID: engineering.StableID(s.project, "placement", device), Maturity: engineering.Estimated}, EquipmentID: device, RackID: rack, StartU: ref.StartU, PositionCode: ref.PositionCode})
		placed[device] = true
	}
	info, e := s.objects.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if e != nil {
		fail(w, 503, "KICKOFF_HISTORY_SOURCE_UNAVAILABLE")
		return
	}
	newID := uuid.NewString()
	copyKey := s.project + "/kickoff/history/" + newID + "/" + digest
	if _, e = s.objects.CopyObject(ctx, minio.CopyDestOptions{Bucket: s.bucket, Object: copyKey}, minio.CopySrcOptions{Bucket: s.bucket, Object: key}); e != nil {
		fail(w, 503, "OBJECT_STORAGE_UNAVAILABLE")
		return
	}
	if _, e = tx.Exec(ctx, `INSERT INTO kickoff_source(id,project_id,filename,object_key,sha256,byte_size,state) VALUES($1,$2,$3,$4,$5,$6,'recognized')`, newID, s.project, name, copyKey, digest, info.Size); e != nil {
		kickoffFailure(w, e)
		return
	}
	source, e := s.ensureKickoffDomain(ctx, tx, subjectOf(ctx).ID)
	if e != nil {
		kickoffFailure(w, e)
		return
	}
	// New location candidates inherit their original explicit input evidence.
	for _, object := range engineering.ModelObjects(&model) {
		if len(object.Version.EvidenceIDs) == 0 {
			eid := engineering.StableID(source, "history-location-evidence", object.Version.EntityID)
			object.Version.EvidenceIDs = []string{eid}
			evidence = append(evidence, engineering.SourceEvidence{ID: eid, EntityID: object.Version.EntityID, Sheet: "[historical-placement-input]", Cell: body.SourceID, Field: "location_reference"})
		}
	}
	proposal.Summary = "历史资料与候选复用；未调用AI"
	if _, e = s.saveEngineeringDraftTx(ctx, tx, source, 1, "agent", subjectOf(ctx).ID, "historical replay: "+revision, engineering.Extraction{Model: model, Evidence: evidence}, proposal); e != nil {
		kickoffFailure(w, e)
		return
	}
	for table, rows := range kickoff.PhysicalInputs(model) {
		if _, e = tx.Exec(ctx, `INSERT INTO kickoff_input_revision(project_id,table_id,version,workspace_revision,rows,source,actor) VALUES($1,$2,1,$3,$4,'历史资料候选复用',$5)`, s.project, table, body.ExpectedRevision+1, jsonBytes(rows), subjectOf(ctx).ID); e != nil {
			kickoffFailure(w, e)
			return
		}
	}
	if _, e = tx.Exec(ctx, `INSERT INTO kickoff_history_import(project_id,original_source_id,original_revision_id,source_id,actor) VALUES($1,$2,$3,$4,$5)`, s.project, body.SourceID, revision, newID, subjectOf(ctx).ID); e != nil {
		kickoffFailure(w, e)
		return
	}
	if _, e = tx.Exec(ctx, `UPDATE kickoff_workspace SET revision=revision+1 WHERE project_id=$1`, s.project); e != nil {
		kickoffFailure(w, e)
		return
	}
	if e = s.kickoffAudit(ctx, tx, "kickoff.history.replayed", newID, map[string]string{"original_source_id": body.SourceID, "original_revision_id": revision}); e != nil {
		kickoffFailure(w, e)
		return
	}
	if e = tx.Commit(ctx); e != nil {
		kickoffFailure(w, e)
		return
	}
	respond(w, 201, map[string]string{"sourceId": newID, "originalRevisionId": revision})
}
