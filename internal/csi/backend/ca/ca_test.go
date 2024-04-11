package ca

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"testing"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

type Foo struct {
	Name string `json:"foo.example.com/name"`
	Age  int    `json:"foo.example.com/age"`
}

func TestFoo(t *testing.T) {
	// Read CA certificate and key files
	caCertFile := "/Users/kevin/workspace/git/k8s-resources/ca-whg-cn.crt"
	caKeyFile := "/Users/kevin/workspace/git/k8s-resources/ca-whg-cn.key"
	caCertPEM, err := os.ReadFile(caCertFile)
	if err != nil {
		t.Fatalf("failed to read CA certificate file: %v", err)
	}
	caKeyPEM, err := os.ReadFile(caKeyFile)
	if err != nil {
		t.Fatalf("failed to read CA key file: %v", err)
	}

	// Parse CA certificate and key
	caCertBlock, _ := pem.Decode(caCertPEM)
	caKeyBlock, _ := pem.Decode(caKeyPEM)
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		t.Fatalf("failed to parse CA certificate: %v", err)
	}
	caKey, err := x509.ParsePKCS8PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		t.Fatalf("failed to parse CA key: %v", err)
	}

	// Generate a new private key for the server certificate
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate server private key: %v", err)
	}

	// Create a new certificate template
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "example.com",
			Organization: []string{"Example Organization"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add IP addresses and/or domain names to the certificate
	serverTemplate.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	serverTemplate.DNSNames = []string{"localhost"}

	// Sign the server certificate using the CA certificate and key
	serverCertDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create server certificate: %v", err)
	}

	serverCert, err := x509.ParseCertificate(serverCertDER)
	if err != nil {
		t.Fatalf("failed to parse server certificate: %v", err)
	}

	// Encode the server certificate to PEM format
	serverCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCertDER,
	})

	certFile := "/tmp/server.crt"
	if err = os.WriteFile(certFile, serverCertPEM, 0644); err != nil {
		t.Fatalf("failed to write server certificate file: %v", err)
	}
	keyFile := "/tmp/server.key"
	serverKeyPEM := x509.MarshalPKCS1PrivateKey(serverKey)
	if err = os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: serverKeyPEM}), 0644); err != nil {
		t.Fatalf("failed to write server key file: %v", err)
	}

	caFile := "/tmp/ca.crt"
	if err = os.WriteFile(caFile, caCertPEM, 0644); err != nil {
		t.Fatalf("failed to write CA certificate file: %v", err)
	}
	cas := []*x509.Certificate{caCert}
	keystoreData, err := pkcs12.Modern.Encode(serverKey, serverCert, cas, "foo")
	if err != nil {
		t.Fatalf("failed to encode PKCS#12 data: %v", err)
	}

	keystore := "/tmp/keystore.p12"

	if err = os.WriteFile(keystore, keystoreData, 0644); err != nil {
		t.Fatalf("failed to write PKCS#12 file: %v", err)
	}

	trustureData, err := pkcs12.Modern.EncodeTrustStore(cas, "foo")
	if err != nil {
		t.Fatalf("failed to encode PKCS#12 data: %v", err)
	}

	truststore := "/tmp/truststore.p12"

	if err = os.WriteFile(truststore, trustureData, 0644); err != nil {
		t.Fatalf("failed to write PKCS#12 file: %v", err)
	}

}
