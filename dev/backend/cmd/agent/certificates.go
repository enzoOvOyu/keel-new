package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"keel.local/dev/internal/flows"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type renewalOptions struct {
	API        string
	Before     time.Duration
	CheckEvery time.Duration
}

func credentialDirectory(dir string) string {
	if target, e := os.Readlink(filepath.Join(dir, "credentials-current")); e == nil {
		return filepath.Join(dir, target)
	}
	return dir
}
func privateWrite(path string, b []byte) error {
	temporary := path + ".tmp"
	if e := os.WriteFile(temporary, b, 0600); e != nil {
		return e
	}
	return os.Rename(temporary, path)
}
func renewalClient(api, dir string) (*http.Client, error) {
	if !strings.HasPrefix(api, "https://") {
		return nil, fmt.Errorf("HTTPS required for certificate renewal")
	}
	pair, e := tls.LoadX509KeyPair(filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem"))
	if e != nil {
		return nil, e
	}
	ca, e := os.ReadFile(filepath.Join(dir, "ca.pem"))
	if e != nil {
		return nil, e
	}
	roots, e := x509.SystemCertPool()
	if e != nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("invalid agent CA")
	}
	return &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{pair}, RootCAs: roots}}}, nil
}
func rotationCall(client *http.Client, endpoint string, body any) (flows.CertificateRotation, error) {
	var out flows.CertificateRotation
	b, e := json.Marshal(body)
	if e != nil {
		return out, e
	}
	response, e := client.Post(endpoint, "application/json", bytes.NewReader(b))
	if e != nil {
		return out, e
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return out, fmt.Errorf("certificate renewal HTTP %d", response.StatusCode)
	}
	var envelope struct {
		Data flows.CertificateRotation `json:"data"`
	}
	if e = json.NewDecoder(response.Body).Decode(&envelope); e != nil {
		return out, e
	}
	return envelope.Data, nil
}
func rotateAgentCredentials(dir string, options renewalOptions) (bool, error) {
	if options.Before <= 0 {
		return false, nil
	}
	stateFile := filepath.Join(dir, "renewal.state")
	var state struct {
		Generation string `json:"generation"`
	}
	if b, e := os.ReadFile(stateFile); e == nil {
		if e = json.Unmarshal(b, &state); e != nil {
			return false, e
		}
		if state.Generation == "" || filepath.Base(state.Generation) != state.Generation {
			return false, fmt.Errorf("invalid credential generation")
		}
		candidate := filepath.Join(dir, "credentials", state.Generation)
		client, e := renewalClient(options.API, candidate)
		if e != nil {
			return false, e
		}
		defer client.CloseIdleConnections()
		out, e := rotationCall(client, strings.TrimRight(options.API, "/")+"/api/v1/flows/agents/certificates/activate", struct{}{})
		if e != nil {
			return false, e
		}
		if out.State != "active" {
			return false, fmt.Errorf("certificate not active")
		}
		temporary := filepath.Join(dir, "credentials-next")
		os.Remove(temporary)
		if e = os.Symlink(filepath.Join("credentials", state.Generation), temporary); e != nil {
			return false, e
		}
		if e = os.Rename(temporary, filepath.Join(dir, "credentials-current")); e != nil {
			return false, e
		}
		if e = os.Remove(stateFile); e != nil {
			return false, e
		}
		os.Remove(filepath.Join(dir, "renewal-key.pem"))
		return true, nil
	}
	current := credentialDirectory(dir)
	pair, e := tls.LoadX509KeyPair(filepath.Join(current, "cert.pem"), filepath.Join(current, "key.pem"))
	if e != nil {
		return false, e
	}
	leaf, e := x509.ParseCertificate(pair.Certificate[0])
	if e != nil {
		return false, e
	}
	if time.Until(leaf.NotAfter) > options.Before {
		return false, nil
	}
	keyFile := filepath.Join(dir, "renewal-key.pem")
	var key any
	if b, e := os.ReadFile(keyFile); e == nil {
		block, _ := pem.Decode(b)
		if block == nil {
			return false, fmt.Errorf("invalid pending renewal key")
		}
		key, e = x509.ParsePKCS8PrivateKey(block.Bytes)
		if e != nil {
			return false, e
		}
	} else if !os.IsNotExist(e) {
		return false, e
	} else {
		k, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			return false, e
		}
		key = k
		b, e := x509.MarshalPKCS8PrivateKey(k)
		if e != nil {
			return false, e
		}
		if e = privateWrite(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: b})); e != nil {
			return false, e
		}
	}
	csr, e := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: leaf.Subject.CommonName}}, key)
	if e != nil {
		return false, e
	}
	client, e := renewalClient(options.API, current)
	if e != nil {
		return false, e
	}
	defer client.CloseIdleConnections()
	out, e := rotationCall(client, strings.TrimRight(options.API, "/")+"/api/v1/flows/agents/certificates/renew", flows.CertificateRenewal{CSR: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csr}))})
	if e != nil {
		return false, e
	}
	block, _ := pem.Decode([]byte(out.Certificate))
	if block == nil {
		return false, fmt.Errorf("invalid renewed certificate")
	}
	newLeaf, e := x509.ParseCertificate(block.Bytes)
	if e != nil {
		return false, e
	}
	generation := newLeaf.SerialNumber.String()
	destination := filepath.Join(dir, "credentials", generation)
	if e = os.MkdirAll(destination, 0700); e != nil {
		return false, e
	}
	private, e := os.ReadFile(keyFile)
	if e != nil {
		return false, e
	}
	for name, b := range map[string][]byte{"key.pem": private, "cert.pem": []byte(out.Certificate), "ca.pem": []byte(out.CA)} {
		if e = privateWrite(filepath.Join(destination, name), b); e != nil {
			return false, e
		}
	}
	if _, e = tls.LoadX509KeyPair(filepath.Join(destination, "cert.pem"), filepath.Join(destination, "key.pem")); e != nil {
		return false, e
	}
	state.Generation = generation
	b, e := json.Marshal(state)
	if e != nil {
		return false, e
	}
	if e = privateWrite(stateFile, b); e != nil {
		return false, e
	}
	return rotateAgentCredentials(dir, options)
}
