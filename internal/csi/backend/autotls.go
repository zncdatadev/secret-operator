package backend

import (
	"context"
	"crypto/x509"
	"time"

	"github.com/zncdatadev/secret-operator/pkg/pod_info"
	"github.com/zncdatadev/secret-operator/pkg/util"
	"github.com/zncdatadev/secret-operator/pkg/volume"
	"sigs.k8s.io/controller-runtime/pkg/client"

	secretsv1alpha1 "github.com/zncdatadev/secret-operator/api/v1alpha1"
	"github.com/zncdatadev/secret-operator/internal/csi/backend/ca"
)

const (
	KeystoreP12FileName   = "keystore.p12"
	TruststoreP12FileName = "truststore.p12"
	PEMTlsCertFileName    = "tls.crt"
	PEMTlsKeyFileName     = "tls.key"
	PEMCaCertFileName     = "ca.crt"
)

type AutoTlsBackend struct {
	client                 client.Client
	podInfo                *pod_info.PodInfo
	volumeSelector         *volume.SecretVolumeSelector
	maxCertificateLifeTime time.Duration

	ca *secretsv1alpha1.CASpec
}

func NewAutoTlsBackend(
	client client.Client,
	podInfo *pod_info.PodInfo,
	volumeSelector *volume.SecretVolumeSelector,
	autotls *secretsv1alpha1.AutoTlsSpec,
) (*AutoTlsBackend, error) {
	maxCertificateLifeTime, err := time.ParseDuration(autotls.MaxCertificateLifeTime)
	if err != nil {
		return nil, err
	}

	return &AutoTlsBackend{
		client:                 client,
		podInfo:                podInfo,
		volumeSelector:         volumeSelector,
		maxCertificateLifeTime: maxCertificateLifeTime,
		ca:                     autotls.CA,
	}, nil
}

func (a *AutoTlsBackend) getCertLife() (time.Duration, error) {
	// TODO: implement
	return time.Duration(10 * time.Hour), nil
}

func (a *AutoTlsBackend) certificateFormat() volume.SecretFormat {
	return a.volumeSelector.Format
}

// Convert the certificate to the format required by the volume
// If the format is PKCS12, the certificate will be converted to PKCS12 format,
// otherwise it will be converted to PEM format.
func (a *AutoTlsBackend) certificateConvert(serverCert *ca.Certificate, caCert *ca.Certificate) (map[string]string, error) {
	format := a.certificateFormat()

	if format == volume.SecretFormatTLSP12 {
		logger.Info("Converting certificate to PKCS12 format")
		password := a.volumeSelector.TlsPKCS12Password
		cas := []*x509.Certificate{caCert.Certificate}

		truststore, err := caCert.TrustStoreP12(password, cas)
		if err != nil {
			return nil, err
		}
		keyStore, err := serverCert.KeyStoreP12(password, cas)
		if err != nil {
			return nil, err
		}
		return map[string]string{
			KeystoreP12FileName:   string(keyStore),
			TruststoreP12FileName: string(truststore),
		}, nil
	}
	logger.Info("Converting certificate to PEM format")
	return map[string]string{
		PEMTlsCertFileName: string(serverCert.CertificatePEM()),
		PEMTlsKeyFileName:  string(serverCert.PrivateKeyPEM()),
		PEMCaCertFileName:  string(caCert.CertificatePEM()),
	}, nil
}

func (a *AutoTlsBackend) GetSecretData(ctx context.Context) (*util.SecretContent, error) {
	certificateAuthority, err := a.getCertificateAuthority(ctx)
	if err != nil {
		return nil, err
	}

	addresses, err := a.getAddresses(ctx)
	if err != nil {
		return nil, err
	}

	duration, err := a.getCertLife()
	if err != nil {
		return nil, err
	}

	notAfter := time.Now().Add(duration)

	// Set empty cnName, then san critical extension forced to be used
	// From RFC 5280, Section 4.2.1.6
	cnName := ""

	logger.Info("Signe certificate", "commonName", cnName, "notAfter", notAfter, "addresses", addresses)
	serverCert, err := certificateAuthority.SignServerCertificate(
		cnName,
		addresses,
		notAfter,
	)

	if err != nil {
		return nil, err
	}

	data, err := a.certificateConvert(serverCert, certificateAuthority.PublicCertificate())
	if err != nil {
		return nil, err
	}

	expiresTime := notAfter

	return &util.SecretContent{
		Data:        data,
		ExpiresTime: &expiresTime,
	}, nil
}

func (a *AutoTlsBackend) getAddresses(ctx context.Context) ([]pod_info.Address, error) {
	return a.podInfo.GetScopedAddresses(ctx)
}

func (a *AutoTlsBackend) SignCertificate(ctx context.Context, ca *ca.CertificateAuthority) error {

	panic("not implemented")

	// ca.SignServerCertificate(
	// 	a.podInfo.GetPodName(),
	// )

}

// Get CAs from the data in the secret, and get an older CA from them.
//
// During the process of getting CAs from secret data, expired CAs will be filtered out.
// If there is no available CA in the end, this situation may be that there is no available data in the secret, or the CA has expired,
// In the case of auto being true, a new CA will be created. Otherwise, return an error.
//
// During the process of getting the certificate, it will check whether the certificate is about to expire,
// and the check condition is whether it has exceeded half of the maximum certificate validity period.
// If it is about to expire, a new certificate will be generated when auto is true.
func (a *AutoTlsBackend) getCertificateAuthority(ctx context.Context) (*ca.CertificateAuthority, error) {

	caCertificateLifeTime, err := time.ParseDuration(a.ca.CACertificateLifeTime)
	if err != nil {
		return nil, err
	}

	certManager, err := ca.NewCertificateManager(
		ctx,
		a.client,
		caCertificateLifeTime,
		a.ca.AutoGenerated,
		a.ca.Secret.Name,
		a.ca.Secret.Namespace,
	)
	if err != nil {
		return nil, err
	}

	atAfter := time.Now().Add(a.maxCertificateLifeTime) // server cert lifetime in secret class configed

	certificateAuthority, err := certManager.GetCertificateAuthority(atAfter)
	if err != nil {
		return nil, err
	}

	return certificateAuthority, nil

}
