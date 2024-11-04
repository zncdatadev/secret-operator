package ca

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"time"

	pkcs12 "software.sslmate.com/src/go-pkcs12"

	"github.com/zncdatadev/secret-operator/pkg/pod_info"
)

type PEMkeyPair struct {
	CertPEMBlock []byte
	KeyPEMBlock  []byte
}

type Certificate struct {
	Certificate *x509.Certificate
	privateKey  *rsa.PrivateKey
}

func (c *Certificate) SerialNumber() string {
	return formatSerialNumber(c.Certificate.SerialNumber)
}

func NewCertificateFromData(certPEM []byte, keyPEM []byte) (*Certificate, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)

	if err != nil {
		return nil, err
	}

	return &Certificate{
		Certificate: cert.Leaf,
		privateKey:  cert.PrivateKey.(*rsa.PrivateKey),
	}, nil
}

func (c *Certificate) GetPrivateKey() *rsa.PrivateKey {
	return c.privateKey
}

func (c *Certificate) CertificatePEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Certificate.Raw})
}

func (c *Certificate) PrivateKeyPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(c.privateKey)})
}

func (c *Certificate) TrustStoreP12(password string, caCerts []*x509.Certificate) ([]byte, error) {
	return pkcs12.Modern.EncodeTrustStore(caCerts, password)
}

func (c *Certificate) KeyStoreP12(password string, caCerts []*x509.Certificate) (pfxData []byte, err error) {
	return pkcs12.Modern.Encode(c.privateKey, c.Certificate, caCerts, password)
}

type CertificateAuthority struct {
	Certificate *x509.Certificate
	privateKey  *rsa.PrivateKey
}

func NewCertificateAuthorityFromData(
	certPEM []byte,
	keyPEM []byte,
) (*CertificateAuthority, error) {
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)

	if err != nil {
		return nil, err
	}

	x509Cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return nil, err
	}

	return NewCertificateAuthority(
		&Certificate{Certificate: x509Cert, privateKey: tlsCert.PrivateKey.(*rsa.PrivateKey)},
	)
}

// NewCertificateAuthorityFromSecret creates a new CertificateAuthority from a secret
func NewCertificateAuthority(root *Certificate) (*CertificateAuthority, error) {
	// check cert is a CA
	if !root.Certificate.IsCA {
		return nil, errors.New("root certificate is not a CA")
	}

	return &CertificateAuthority{
		Certificate: root.Certificate,
		privateKey:  root.privateKey,
	}, nil
}

func (c *CertificateAuthority) SerialNumber() string {
	return formatSerialNumber(c.Certificate.SerialNumber)
}

func (c *CertificateAuthority) PublicCertificate() *Certificate {
	return &Certificate{
		Certificate: c.Certificate,
		privateKey:  nil,
	}
}

func (c *CertificateAuthority) privateKeyPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(c.privateKey)})
}

func (c *CertificateAuthority) CertificatePEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Certificate.Raw})
}

func (c *CertificateAuthority) SignCertificate(
	addresses []pod_info.Address,
	extKeyUsage []x509.ExtKeyUsage,
	notAfter time.Time) (*Certificate, error) {
	// Generate a new private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	publicKeySum, err := publicKeySHA256(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}

	serialNumber, err := generateSerialNumber()
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		Subject:               pkix.Name{CommonName: "generated certificate for pod"},
		IsCA:                  false,
		BasicConstraintsValid: true,
		SerialNumber:          serialNumber,
		Issuer:                c.Certificate.Subject,
		SubjectKeyId:          publicKeySum[:],
		AuthorityKeyId:        c.Certificate.SubjectKeyId,
		PublicKey:             &privateKey.PublicKey,
		NotBefore:             time.Now(),
		NotAfter:              notAfter,
		// see http://golang.org/pkg/crypto/x509/#KeyUsage
		KeyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,

		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	if extKeyUsage != nil {
		template.ExtKeyUsage = extKeyUsage
	}

	for _, address := range addresses {
		if address.IP != nil {
			template.IPAddresses = append(template.IPAddresses, address.IP)
		}
		if address.Hostname != "" {
			template.DNSNames = append(template.DNSNames, address.Hostname)
		}
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, c.Certificate, &privateKey.PublicKey, c.privateKey)
	if err != nil {
		return nil, err
	}

	// Parse the resulting certificate so we can use it again
	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, err
	}

	logger.V(0).Info("Signed certificate", "subject", cert.Subject, "serialNumber", formatSerialNumber(cert.SerialNumber), "notAfter", cert.NotAfter, "sanDns", cert.DNSNames, "sanIp", cert.IPAddresses)
	return &Certificate{
		Certificate: cert,
		privateKey:  privateKey,
	}, nil
}

func (c *CertificateAuthority) SignServerCertificate(
	addresses []pod_info.Address,
	notAfter time.Time,
) (*Certificate, error) {
	return c.SignCertificate(addresses, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, notAfter)
}

func (c *CertificateAuthority) SignClientCertificate(
	addresses []pod_info.Address,
	notAfter time.Time,
) (*Certificate, error) {
	return c.SignCertificate(addresses, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, notAfter)
}

func (c *CertificateAuthority) Rotate(notAfter time.Time) (*CertificateAuthority, error) {
	newCA, err := NewSelfSignedCertificateAuthority(notAfter, c.Certificate, c.privateKey)
	if err != nil {
		return nil, err
	}

	logger.V(0).Info("Rotated certificate authority", "notAfter", newCA.Certificate.NotAfter, "newSerialNumber", newCA.SerialNumber(), "currentSerialNumber", c.SerialNumber())
	return newCA, nil
}

func NewSelfSignedCertificateAuthority(expeiry time.Time, parent *x509.Certificate, parentPrivateKey *rsa.PrivateKey) (*CertificateAuthority, error) {
	// Generate a new private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	publicKeySum, err := publicKeySHA256(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}

	subectName := pkix.Name{
		CommonName: "secret-operator self-signed CA",
	}

	serialNumber, err := generateSerialNumber()
	if err != nil {
		return nil, err
	}

	// Create a self-signed certificate
	template := &x509.Certificate{
		IsCA:                  true,
		BasicConstraintsValid: true,
		SerialNumber:          serialNumber,
		Subject:               subectName,
		SubjectKeyId:          publicKeySum[:],
		Issuer:                subectName,
		AuthorityKeyId:        publicKeySum[:],
		PublicKey:             &privateKey.PublicKey,
		NotBefore:             time.Now(),
		NotAfter:              expeiry,
		// see http://golang.org/pkg/crypto/x509/#KeyUsage
		KeyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	if parent == nil {
		parent = template
	}

	if parentPrivateKey == nil {
		parentPrivateKey = privateKey
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, parent, &privateKey.PublicKey, parentPrivateKey)
	if err != nil {
		return nil, err
	}

	// Parse the resulting certificate so we can use it again
	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, err
	}

	return NewCertificateAuthority(
		&Certificate{Certificate: cert, privateKey: privateKey},
	)
}

// generate a 64-bit serial number
func generateSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 64)
	return rand.Int(rand.Reader, serialNumberLimit)
}

// Compute the SHA-256 hash of the public key
func publicKeySHA256(publicKey *rsa.PublicKey) ([]byte, error) {
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(publicKeyBytes)

	return sum[:], nil
}

var (
	re = regexp.MustCompile("(?i)([0-9a-f]{2})")
)

func formatSerialNumber(serialNumber *big.Int) string {
	// convert the serial number to a hex string
	hexStr := fmt.Sprintf("%x", serialNumber)

	// insert a '-' every two characters
	formattedStr := re.ReplaceAllString(hexStr, "$1-")

	// delete the last '-'
	if len(formattedStr) > 0 && formattedStr[len(formattedStr)-1] == '-' {
		formattedStr = formattedStr[:len(formattedStr)-1]
	}

	return formattedStr
}
