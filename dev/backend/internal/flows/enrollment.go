package flows

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"github.com/google/uuid"
	"math/big"
	"net/url"
	"os"
	"time"
)

type Agent struct {
	ID           string            `json:"id"`
	Project      string            `json:"project_id"`
	Asset        string            `json:"asset_id"`
	Identity     map[string]string `json:"identity"`
	Version      string            `json:"version"`
	Capabilities []string          `json:"capabilities"`
	State        string            `json:"state"`
	Serial       *string           `json:"certificate_serial"`
	Expires      *time.Time        `json:"certificate_expires_at"`
	Heartbeat    *time.Time        `json:"last_heartbeat"`
	Online       bool              `json:"online"`
}
type EnrollInput struct {
	Token        string            `json:"token"`
	CSR          string            `json:"csr"`
	Identity     map[string]string `json:"identity"`
	Version      string            `json:"version"`
	Capabilities []string          `json:"capabilities"`
}

func (s *Service) Agents(ctx context.Context, p Scope, paging ...url.Values) ([]Agent, error) {
	limit, offset, e := collectionWindow(paging)
	if e != nil {
		return nil, e
	}
	rows, e := s.DB.Query(ctx, `SELECT id::text,project_id,asset_id,identity,version,capabilities,state,certificate_serial,certificate_expires_at,last_heartbeat,COALESCE(state='enrolled' AND last_heartbeat>now()-interval '45 seconds' AND certificate_expires_at>now(),false) FROM flow_agent WHERE project_id=ANY($1) ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, p.Projects, limit, offset)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Agent{}
	for rows.Next() {
		var a Agent
		var identity, caps []byte
		if e = rows.Scan(&a.ID, &a.Project, &a.Asset, &identity, &a.Version, &caps, &a.State, &a.Serial, &a.Expires, &a.Heartbeat, &a.Online); e != nil {
			return nil, e
		}
		json.Unmarshal(identity, &a.Identity)
		json.Unmarshal(caps, &a.Capabilities)
		if a.Capabilities == nil {
			a.Capabilities = []string{}
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Service) Token(ctx context.Context, p Scope, project, asset string) (map[string]any, error) {
	if e := requireProject(p, project); e != nil {
		return nil, e
	}
	targets, e := s.Targets(ctx, Scope{Projects: []string{project}}, "")
	if e != nil {
		return nil, e
	}
	found := false
	for _, t := range targets {
		if t.ID == asset && t.Type != "switch" && !Maintained(t) {
			found = true
		}
	}
	if !found {
		return nil, problem(422, "LINUX_TARGET_REQUIRED")
	}
	if os.Getenv("KEEL_AGENT_CA_CERT") == "" {
		return nil, problem(503, "AGENT_CA_NOT_CONFIGURED")
	}
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)
	expires := time.Now().Add(15 * time.Minute)
	id := uuid.NewString()
	_, e = s.DB.Exec(ctx, `INSERT INTO flow_enrollment(id,project_id,asset_id,token_hash,expires_at,created_by) VALUES($1,$2,$3,$4,$5,$6)`, id, project, asset, hash(token), expires, p.User)
	if e != nil {
		return nil, e
	}
	s.audit(ctx, p, project, "agent.enrollment", asset, map[string]any{"expires_at": expires})
	return map[string]any{"id": id, "token": token, "expires_at": expires, "asset_id": asset}, nil
}
func (s *Service) Enroll(ctx context.Context, input EnrollInput) (map[string]any, error) {
	if input.Version == "" || len(input.Capabilities) == 0 {
		return nil, problem(422, "AGENT_VERSION_AND_CAPABILITIES_REQUIRED")
	}
	block, _ := pem.Decode([]byte(input.CSR))
	if block == nil {
		return nil, problem(422, "CSR_INVALID")
	}
	csr, e := x509.ParseCertificateRequest(block.Bytes)
	if e != nil || csr.CheckSignature() != nil {
		return nil, problem(422, "CSR_INVALID")
	}
	caBytes, e := os.ReadFile(os.Getenv("KEEL_AGENT_CA_CERT"))
	if e != nil {
		return nil, problem(503, "AGENT_CA_NOT_CONFIGURED")
	}
	caBlock, _ := pem.Decode(caBytes)
	if caBlock == nil {
		return nil, problem(503, "AGENT_CA_INVALID")
	}
	ca, e := x509.ParseCertificate(caBlock.Bytes)
	if e != nil {
		return nil, e
	}
	keyBytes, e := os.ReadFile(os.Getenv("KEEL_AGENT_CA_KEY"))
	if e != nil {
		return nil, e
	}
	keyBlock, _ := pem.Decode(keyBytes)
	if keyBlock == nil {
		return nil, problem(503, "AGENT_CA_INVALID")
	}
	key, e := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if e != nil {
		return nil, e
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, problem(503, "AGENT_CA_INVALID")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	var tokenID, project, asset string
	e = tx.QueryRow(ctx, `SELECT id::text,project_id,asset_id FROM flow_enrollment WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at>now() FOR UPDATE`, hash(input.Token)).Scan(&tokenID, &project, &asset)
	if e != nil {
		return nil, problem(403, "ENROLLMENT_TOKEN_INVALID")
	}
	var payload []byte
	e = tx.QueryRow(ctx, `SELECT payload || jsonb_build_object('maintenance_state',CASE WHEN maintenance_state='maintained' OR payload->>'maintenance_state'='maintained' OR payload->'fields'->>'maintenance_state'='maintained' THEN 'maintained' ELSE maintenance_state END,'lifecycle_state',CASE WHEN lifecycle_state='maintained' OR payload->>'lifecycle_state'='maintained' OR payload->'fields'->>'lifecycle_state'='maintained' THEN 'maintained' ELSE lifecycle_state END,'serial',COALESCE(serial,payload->>'serial')) FROM cmdb_entity WHERE item_uid=$1 AND EXISTS(SELECT 1 FROM cmdb_asset_scope WHERE asset_id=item_uid AND project_id=$2)`, asset, project).Scan(&payload)
	if e != nil {
		return nil, e
	}
	var target Target
	json.Unmarshal(payload, &target.Fields)
	if Maintained(target) {
		return nil, problem(422, "MAINTAINED_IPMI_ONLY")
	}
	matched := false
	conflict := false
	for _, name := range []string{"machine_id", "dmi_uuid", "serial"} {
		expected := field(target, name)
		actual := input.Identity[name]
		if expected != "" && actual != "" {
			if expected == actual {
				matched = true
			} else {
				conflict = true
			}
		}
	}
	state := "pending_verification"
	if matched && !conflict {
		state = "enrolled"
	}
	id := uuid.NewString()
	var existingID, existingState string
	if err := tx.QueryRow(ctx, `SELECT id::text,state FROM flow_agent WHERE project_id=$1 AND asset_id=$2 FOR UPDATE`, project, asset).Scan(&existingID, &existingState); err == nil {
		if existingState == "revoked" {
			return nil, problem(409, "AGENT_REVOKED_REQUIRES_NEW_AUTHORIZATION")
		}
		id = existingID
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	expires := time.Now().Add(30 * 24 * time.Hour)
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: id}, NotBefore: time.Now().Add(-time.Minute), NotAfter: expires, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	der, e := x509.CreateCertificate(rand.Reader, template, ca, csr.PublicKey, signer)
	if e != nil {
		return nil, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO flow_agent(id,project_id,asset_id,identity,version,capabilities,state,certificate_serial,certificate_expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(project_id,asset_id) DO UPDATE SET identity=EXCLUDED.identity,version=EXCLUDED.version,capabilities=EXCLUDED.capabilities,state=EXCLUDED.state,certificate_serial=EXCLUDED.certificate_serial,certificate_expires_at=EXCLUDED.certificate_expires_at,last_heartbeat=NULL`, id, project, asset, raw(input.Identity), input.Version, raw(input.Capabilities), state, serial.String(), expires)
	if e != nil {
		return nil, problem(409, "AGENT_ALREADY_REGISTERED")
	}
	_, e = tx.Exec(ctx, `UPDATE flow_enrollment SET consumed_at=now() WHERE id=$1`, tokenID)
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return map[string]any{"id": id, "state": state, "certificate": string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), "ca": string(caBytes), "expires_at": expires}, nil
}
