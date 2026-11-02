package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"keel.local/dev/internal/flows"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRenewalRecoversFailedActivationWithoutLosingCurrentKey(t *testing.T) {
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Synthetic test CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(365 * 24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDer, e := x509.CreateCertificate(rand.Reader, template, template, &caKey.PublicKey, caKey)
	if e != nil {
		t.Fatal(e)
	}
	ca, _ := x509.ParseCertificate(caDer)
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDer})
	leaf := func(serial int64, server bool, key *ecdsa.PrivateKey) ([]byte, []byte) {
		usage := x509.ExtKeyUsageClientAuth
		if server {
			usage = x509.ExtKeyUsageServerAuth
		}
		der, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "synthetic-agent"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), ExtKeyUsage: []x509.ExtKeyUsage{usage}, KeyUsage: x509.KeyUsageDigitalSignature}, ca, &key.PublicKey, caKey)
		if err != nil {
			t.Fatal(err)
		}
		private, _ := x509.MarshalPKCS8PrivateKey(key)
		return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private})
	}
	dir := t.TempDir()
	oldKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	oldCert, oldPrivate := leaf(2, false, oldKey)
	for name, b := range map[string][]byte{"cert.pem": oldCert, "key.pem": oldPrivate, "ca.pem": caPEM} {
		os.WriteFile(filepath.Join(dir, name), b, 0600)
	}
	prepared, activated := 0, 0
	var nextCertificate string
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 {
			http.Error(w, "mTLS required", 403)
			return
		}
		var out flows.CertificateRotation
		if r.URL.Path == "/api/v1/flows/agents/certificates/renew" {
			prepared++
			var input flows.CertificateRenewal
			json.NewDecoder(r.Body).Decode(&input)
			block, _ := pem.Decode([]byte(input.CSR))
			csr, _ := x509.ParseCertificateRequest(block.Bytes)
			der, e := x509.CreateCertificate(rand.Reader, &x509.Certificate{SerialNumber: big.NewInt(3), Subject: pkix.Name{CommonName: "synthetic-agent"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(30 * 24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}, ca, csr.PublicKey, caKey)
			if e != nil {
				t.Error(e)
			}
			nextCertificate = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
			out = flows.CertificateRotation{ID: "synthetic", AgentID: "synthetic-agent", Certificate: nextCertificate, CA: string(caPEM), State: "pending"}
		} else {
			activated++
			if r.TLS.PeerCertificates[0].SerialNumber.Int64() != 3 {
				t.Error("activation did not prove possession of new key")
			}
			if activated == 1 {
				http.Error(w, "temporary activation failure", 503)
				return
			}
			out = flows.CertificateRotation{ID: "synthetic", State: "active"}
		}
		json.NewEncoder(w).Encode(map[string]any{"data": out})
	}))
	serverKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	serverCert, serverPrivate := leaf(4, true, serverKey)
	pair, _ := tls.X509KeyPair(serverCert, serverPrivate)
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{pair}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots, MinVersion: tls.VersionTLS13}
	server.StartTLS()
	defer server.Close()
	options := renewalOptions{API: server.URL, Before: 24 * time.Hour}
	if changed, e := rotateAgentCredentials(dir, options); e == nil || changed {
		t.Fatal("failed activation did not retain pending state")
	}
	if credentialDirectory(dir) != dir {
		t.Fatal("failed activation changed current generation")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "key.pem")); string(b) != string(oldPrivate) {
		t.Fatal("old key was overwritten")
	}
	changed, e := rotateAgentCredentials(dir, options)
	if e != nil || !changed {
		t.Fatal("pending activation did not recover", e)
	}
	if prepared != 1 || activated != 2 {
		t.Fatalf("unexpected renewal replay %d/%d", prepared, activated)
	}
	current := credentialDirectory(dir)
	if _, e = tls.LoadX509KeyPair(filepath.Join(current, "cert.pem"), filepath.Join(current, "key.pem")); e != nil {
		t.Fatal("new generation is not a valid pair", e)
	}
	if _, e = os.Stat(filepath.Join(dir, "renewal.state")); !os.IsNotExist(e) {
		t.Fatal("pending state retained after activation")
	}
}
