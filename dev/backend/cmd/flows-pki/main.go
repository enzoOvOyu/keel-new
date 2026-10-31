// Creates a development-only CA/server certificate in an explicitly supplied private directory.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

func main() {
	dir := flag.String("dir", "", "private output directory")
	host := flag.String("host", "localhost", "gateway DNS name")
	flag.Parse()
	if *dir == "" {
		panic("dir required")
	}
	if _, e := os.Stat(filepath.Join(*dir, "ca-key.pem")); e == nil {
		panic("refusing to overwrite CA")
	}
	must(os.MkdirAll(*dir, 0700))
	caKey, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(e)
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: big.NewInt(now.UnixNano()), Subject: pkix.Name{CommonName: "Keel isolated development CA"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(365 * 24 * time.Hour), KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign, BasicConstraintsValid: true, IsCA: true}
	caDER, e := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	must(e)
	write(*dir, "ca.pem", "CERTIFICATE", caDER)
	priv, e := x509.MarshalPKCS8PrivateKey(caKey)
	must(e)
	write(*dir, "ca-key.pem", "PRIVATE KEY", priv)
	serverKey, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(e)
	server := &x509.Certificate{SerialNumber: big.NewInt(now.UnixNano() + 1), Subject: pkix.Name{CommonName: *host}, DNSNames: []string{*host}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: ca.NotBefore, NotAfter: now.Add(90 * 24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, e := x509.CreateCertificate(rand.Reader, server, ca, &serverKey.PublicKey, caKey)
	must(e)
	write(*dir, "server.pem", "CERTIFICATE", der)
	priv, e = x509.MarshalPKCS8PrivateKey(serverKey)
	must(e)
	write(*dir, "server-key.pem", "PRIVATE KEY", priv)
	fmt.Println("Created development PKI; no private keys printed")
}
func must(e error) {
	if e != nil {
		panic(e)
	}
}
func write(dir, name, kind string, b []byte) {
	must(os.WriteFile(filepath.Join(dir, name), pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: b}), 0600))
}
