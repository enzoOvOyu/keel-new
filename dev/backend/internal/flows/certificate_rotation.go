package flows

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"math/big"
	"os"
	"time"
)

type CertificateRotation struct {
	ID          string    `json:"id"`
	AgentID     string    `json:"agent_id"`
	Certificate string    `json:"certificate"`
	CA          string    `json:"ca"`
	Expires     time.Time `json:"expires_at"`
	State       string    `json:"state"`
}
type CertificateRenewal struct {
	CSR string `json:"csr"`
}

func agentIssuer() (*x509.Certificate, crypto.Signer, []byte, error) {
	caBytes, e := os.ReadFile(os.Getenv("KEEL_AGENT_CA_CERT"))
	if e != nil {
		return nil, nil, nil, problem(503, "AGENT_CA_NOT_CONFIGURED")
	}
	block, _ := pem.Decode(caBytes)
	if block == nil {
		return nil, nil, nil, problem(503, "AGENT_CA_INVALID")
	}
	ca, e := x509.ParseCertificate(block.Bytes)
	if e != nil {
		return nil, nil, nil, e
	}
	keyBytes, e := os.ReadFile(os.Getenv("KEEL_AGENT_CA_KEY"))
	if e != nil {
		return nil, nil, nil, problem(503, "AGENT_CA_NOT_CONFIGURED")
	}
	keyBlock, _ := pem.Decode(keyBytes)
	if keyBlock == nil {
		return nil, nil, nil, problem(503, "AGENT_CA_INVALID")
	}
	key, e := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if e != nil {
		return nil, nil, nil, e
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, nil, nil, problem(503, "AGENT_CA_INVALID")
	}
	return ca, signer, caBytes, nil
}
func verifyAgentCertificate(peer *x509.Certificate, ca *x509.Certificate) error {
	if peer == nil || peer.SerialNumber == nil || peer.Subject.CommonName == "" {
		return problem(403, "AGENT_CLIENT_CERTIFICATE_REQUIRED")
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	if _, e := peer.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); e != nil {
		return problem(403, "AGENT_CLIENT_CERTIFICATE_INVALID")
	}
	return nil
}
func scanRotation(row pgx.Row) (CertificateRotation, error) {
	var out CertificateRotation
	e := row.Scan(&out.ID, &out.AgentID, &out.Certificate, &out.CA, &out.Expires, &out.State)
	return out, e
}

const rotationColumns = `id::text,agent_id::text,certificate,ca,expires_at,state`

func (s *Service) PrepareCertificateRenewal(ctx context.Context, peer *x509.Certificate, input CertificateRenewal) (CertificateRotation, error) {
	ca, signer, caBytes, e := agentIssuer()
	if e != nil {
		return CertificateRotation{}, e
	}
	if e = verifyAgentCertificate(peer, ca); e != nil {
		return CertificateRotation{}, e
	}
	block, _ := pem.Decode([]byte(input.CSR))
	if block == nil {
		return CertificateRotation{}, problem(422, "CSR_INVALID")
	}
	csr, e := x509.ParseCertificateRequest(block.Bytes)
	if e != nil || csr.CheckSignature() != nil {
		return CertificateRotation{}, problem(422, "CSR_INVALID")
	}
	public, e := x509.MarshalPKIXPublicKey(csr.PublicKey)
	if e != nil {
		return CertificateRotation{}, e
	}
	digest := hash(string(public))
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return CertificateRotation{}, e
	}
	defer tx.Rollback(ctx)
	var project, serial, state string
	var maintained bool
	e = tx.QueryRow(ctx, `SELECT a.project_id,a.certificate_serial,a.state,COALESCE(c.lifecycle_state='maintained' OR c.maintenance_state='maintained' OR c.payload->>'lifecycle_state'='maintained' OR c.payload->>'maintenance_state'='maintained',false) FROM flow_agent a JOIN cmdb_entity c ON c.item_uid=a.asset_id WHERE a.id::text=$1 FOR UPDATE OF a`, peer.Subject.CommonName).Scan(&project, &serial, &state, &maintained)
	if e != nil || state != "enrolled" || serial != peer.SerialNumber.String() || maintained {
		return CertificateRotation{}, problem(403, "AGENT_RENEWAL_NOT_AUTHORIZED")
	}
	out, e := scanRotation(tx.QueryRow(ctx, `SELECT `+rotationColumns+` FROM flow_agent_certificate_rotation WHERE agent_id::text=$1 AND state='pending'`, peer.Subject.CommonName))
	if e == nil {
		var prior string
		if e = tx.QueryRow(ctx, `SELECT public_key_hash FROM flow_agent_certificate_rotation WHERE id=$1`, out.ID).Scan(&prior); e != nil {
			return out, e
		}
		if prior != digest {
			return out, problem(409, "CERTIFICATE_ROTATION_PENDING_KEY_CONFLICT")
		}
		return out, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return CertificateRotation{}, e
	}
	var active bool
	if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM flow_agent_task WHERE agent_id::text=$1 AND state IN('DISPATCHED','RUNNING','RECONCILING'))`, peer.Subject.CommonName).Scan(&active); e != nil {
		return CertificateRotation{}, e
	}
	if active {
		return CertificateRotation{}, problem(409, "AGENT_ACTIVE_TASKS_BLOCK_ROTATION")
	}
	number, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if e != nil {
		return CertificateRotation{}, e
	}
	expires := time.Now().Add(30 * 24 * time.Hour)
	if ca.NotAfter.Before(expires) {
		expires = ca.NotAfter
	}
	template := &x509.Certificate{SerialNumber: number, Subject: pkix.Name{CommonName: peer.Subject.CommonName}, NotBefore: time.Now().Add(-time.Minute), NotAfter: expires, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	der, e := x509.CreateCertificate(rand.Reader, template, ca, csr.PublicKey, signer)
	if e != nil {
		return CertificateRotation{}, e
	}
	out = CertificateRotation{ID: uuid.NewString(), AgentID: peer.Subject.CommonName, Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), CA: string(caBytes), Expires: expires, State: "pending"}
	_, e = tx.Exec(ctx, `INSERT INTO flow_agent_certificate_rotation(id,agent_id,old_serial,new_serial,public_key_hash,certificate,ca,expires_at,state) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'pending')`, out.ID, out.AgentID, serial, number.String(), digest, out.Certificate, out.CA, expires)
	if e != nil {
		return out, e
	}
	if e = tx.Commit(ctx); e != nil {
		return out, e
	}
	s.audit(ctx, Scope{Actor: "agent:" + out.AgentID}, project, "agent.certificate.prepare", out.AgentID, map[string]any{"rotation_id": out.ID, "expires_at": expires})
	return out, nil
}
func (s *Service) ActivateCertificateRenewal(ctx context.Context, peer *x509.Certificate) (CertificateRotation, error) {
	ca, _, _, e := agentIssuer()
	if e != nil {
		return CertificateRotation{}, e
	}
	if e = verifyAgentCertificate(peer, ca); e != nil {
		return CertificateRotation{}, e
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return CertificateRotation{}, e
	}
	defer tx.Rollback(ctx)
	var project, serial, state string
	var maintained bool
	e = tx.QueryRow(ctx, `SELECT a.project_id,a.certificate_serial,a.state,COALESCE(c.lifecycle_state='maintained' OR c.maintenance_state='maintained' OR c.payload->>'lifecycle_state'='maintained' OR c.payload->>'maintenance_state'='maintained',false) FROM flow_agent a JOIN cmdb_entity c ON c.item_uid=a.asset_id WHERE a.id::text=$1 FOR UPDATE OF a`, peer.Subject.CommonName).Scan(&project, &serial, &state, &maintained)
	if e != nil || state != "enrolled" || maintained {
		return CertificateRotation{}, problem(403, "AGENT_RENEWAL_NOT_AUTHORIZED")
	}
	out, e := scanRotation(tx.QueryRow(ctx, `SELECT `+rotationColumns+` FROM flow_agent_certificate_rotation WHERE agent_id::text=$1 AND new_serial=$2 AND state IN('pending','active') FOR UPDATE`, peer.Subject.CommonName, peer.SerialNumber.String()))
	if e != nil {
		return out, problem(403, "CERTIFICATE_ROTATION_NOT_FOUND")
	}
	if out.State == "active" && serial == peer.SerialNumber.String() {
		return out, nil
	}
	var oldSerial string
	if e = tx.QueryRow(ctx, `SELECT old_serial FROM flow_agent_certificate_rotation WHERE id=$1`, out.ID).Scan(&oldSerial); e != nil {
		return out, e
	}
	if serial != oldSerial {
		return out, problem(409, "CERTIFICATE_ROTATION_SUPERSEDED")
	}
	var active bool
	if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM flow_agent_task WHERE agent_id::text=$1 AND state IN('DISPATCHED','RUNNING','RECONCILING'))`, out.AgentID).Scan(&active); e != nil {
		return out, e
	}
	if active {
		return out, problem(409, "AGENT_ACTIVE_TASKS_BLOCK_ROTATION")
	}
	if _, e = tx.Exec(ctx, `UPDATE flow_agent SET certificate_serial=$2,certificate_expires_at=$3 WHERE id::text=$1`, out.AgentID, peer.SerialNumber.String(), out.Expires); e != nil {
		return out, e
	}
	if _, e = tx.Exec(ctx, `UPDATE flow_agent_certificate_rotation SET state='active',activated_at=now() WHERE id=$1`, out.ID); e != nil {
		return out, e
	}
	if e = tx.Commit(ctx); e != nil {
		return out, e
	}
	out.State = "active"
	s.audit(ctx, Scope{Actor: "agent:" + out.AgentID}, project, "agent.certificate.activate", out.AgentID, map[string]any{"rotation_id": out.ID, "old_serial": oldSerial, "new_serial": peer.SerialNumber.String()})
	return out, nil
}
