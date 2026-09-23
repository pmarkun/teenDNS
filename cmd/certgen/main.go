package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

func main() {
	out := flag.String("out", ".local/certs", "output directory")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o700); err != nil {
		log.Fatal(err)
	}
	if err := os.Chmod(*out, 0o755); err != nil {
		log.Fatal(err)
	}

	now := time.Now()
	caKey := mustKey()
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "teenDNS laboratory CA"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(2, 0, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		log.Fatal(err)
	}
	caCertificate, err := x509.ParseCertificate(caDER)
	if err != nil {
		log.Fatal(err)
	}

	serverKey := mustKey()
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "*.dns.teendns.test"},
		DNSNames:     []string{"*.dns.teendns.test"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCertificate, &serverKey.PublicKey, caKey)
	if err != nil {
		log.Fatal(err)
	}

	writePEM(filepath.Join(*out, "ca.pem"), "CERTIFICATE", caDER, 0o644)
	writePEM(filepath.Join(*out, "server.pem"), "CERTIFICATE", serverDER, 0o644)
	serverKeyDER, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		log.Fatal(err)
	}
	// The key is used only by the isolated Docker laboratory and must be
	// readable by the unprivileged gateway container. Production keys are
	// provisioned separately and must use stricter permissions.
	writePEM(filepath.Join(*out, "server-key.pem"), "EC PRIVATE KEY", serverKeyDER, 0o644)
}

func mustKey() *ecdsa.PrivateKey {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	return key
}

func writePEM(path, blockType string, contents []byte, mode os.FileMode) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	if err := file.Chmod(mode); err != nil {
		log.Fatal(err)
	}
	if err := pem.Encode(file, &pem.Block{Type: blockType, Bytes: contents}); err != nil {
		log.Fatal(err)
	}
}
