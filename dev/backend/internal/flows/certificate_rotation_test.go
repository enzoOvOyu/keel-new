package flows

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCertificateRotationAuthorizationAndActivation(t *testing.T) {
	dsn := os.Getenv("FLOWS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated integration DB not configured")
	}
	if !strings.Contains(dsn, "keel_flows_dev") {
		t.Fatal("isolated DB required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var project string
	if e = db.QueryRow(ctx, `SELECT id FROM project ORDER BY id LIMIT 1`).Scan(&project); e != nil {
		t.Fatal(e)
	}
	id, asset := uuid.NewString(), "synthetic-cert-"+uuid.NewString()
	if _, e = db.Exec(ctx, `INSERT INTO cmdb_entity(item_uid,payload,asset_type,name,source_type,source_ref,maintenance_state,lifecycle_state) VALUES($1,'{}','server','Synthetic rotation target','manual','synthetic-cert','active','discovered')`, asset); e != nil {
		t.Fatal(e)
	}
	defer db.Exec(context.Background(), `DELETE FROM cmdb_entity WHERE item_uid=$1`, asset)
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Synthetic CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(365 * 24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDer, e := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if e != nil {
		t.Fatal(e)
	}
	ca, _ := x509.ParseCertificate(caDer)
	keyBytes, _ := x509.MarshalPKCS8PrivateKey(caKey)
	dir := t.TempDir()
	caFile, keyFile := filepath.Join(dir, "ca.pem"), filepath.Join(dir, "ca-key.pem")
	os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDer}), 0600)
	os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes}), 0600)
	t.Setenv("KEEL_AGENT_CA_CERT", caFile)
	t.Setenv("KEEL_AGENT_CA_KEY", keyFile)
	oldKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	oldDer, e := x509.CreateCertificate(rand.Reader, &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: id}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}, ca, &oldKey.PublicKey, caKey)
	if e != nil {
		t.Fatal(e)
	}
	old, _ := x509.ParseCertificate(oldDer)
	if _, e = db.Exec(ctx, `INSERT INTO flow_agent(id,project_id,asset_id,identity,version,capabilities,state,certificate_serial,certificate_expires_at) VALUES($1,$2,$3,'{}','0.2.0','[]','enrolled','2',$4)`, id, project, asset, old.NotAfter); e != nil {
		t.Fatal(e)
	}
	defer db.Exec(context.Background(), `DELETE FROM flow_agent WHERE id=$1`, id)
	defer db.Exec(context.Background(), `DELETE FROM flow_agent_certificate_rotation WHERE agent_id=$1`, id)
	newKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrDer, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, newKey)
	input := CertificateRenewal{CSR: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDer}))}
	s := &Service{DB: db}
	prepared, e := s.PrepareCertificateRenewal(ctx, old, input)
	if e != nil || prepared.State != "pending" {
		t.Fatalf("prepare %+v %v", prepared, e)
	}
	same, e := s.PrepareCertificateRenewal(ctx, old, input)
	if e != nil || same.ID != prepared.ID {
		t.Fatal("prepare retry lost pending rotation", e)
	}
	var serial string
	if e = db.QueryRow(ctx, `SELECT certificate_serial FROM flow_agent WHERE id=$1`, id).Scan(&serial); e != nil || serial != "2" {
		t.Fatal("old cert invalidated before activation")
	}
	if _, e = s.ActivateCertificateRenewal(ctx, old); e == nil {
		t.Fatal("old certificate activated a new identity")
	}
	block, _ := pem.Decode([]byte(prepared.Certificate))
	newCert, _ := x509.ParseCertificate(block.Bytes)
	activated, e := s.ActivateCertificateRenewal(ctx, newCert)
	if e != nil || activated.State != "active" {
		t.Fatalf("activate %+v %v", activated, e)
	}
	again, e := s.ActivateCertificateRenewal(ctx, newCert)
	if e != nil || again.ID != activated.ID {
		t.Fatal("activation retry is not idempotent", e)
	}
	if _, e = s.PrepareCertificateRenewal(ctx, old, input); e == nil {
		t.Fatal("retired serial accepted")
	}
	if _, e = db.Exec(ctx, `UPDATE cmdb_entity SET maintenance_state='maintained' WHERE item_uid=$1`, asset); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ActivateCertificateRenewal(ctx, newCert); e == nil {
		t.Fatal("maintained agent accepted certificate control")
	}
}
