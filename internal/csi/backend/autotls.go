package backend

import (
	"context"
	"crypto/x509"
	"fmt"
	"math/rand"
	"strings"
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

const (
	DefaultCertLifeTime time.Duration = 24 * 7 * time.Hour
	DefaultCertJitter   float64       = 0.2
	DefaultCertBuffer   time.Duration = 8 * time.Hour
)

var _ IBackend = &AutoTlsBackend{}

type AutoTlsBackend struct {
	client                 client.Client
	podInfo                *pod_info.PodInfo
	volumeContext          *volume.SecretVolumeContext
	maxCertificateLifeTime time.Duration

	ca          *secretsv1alpha1.CASpec
	certManager ca.CertificateManager
}

func NewAutoTlsBackend(config *BackendConfig) (IBackend, error) {
	autotls := config.SecretClass.Spec.Backend.AutoTls
	// max certificate lifetime come from secret class
	// it is used to calculate the certificate lifetime when generate the certificate
	maxCertificateLifeTime, err := time.ParseDuration(autotls.MaxCertificateLifeTime)
	if err != nil {
		return nil, err
	}

	// ca certificate life time come from secret class
	// it is used to calculate the ca certificate lifetime when generate the ca certificate
	caCertificateLifeTime, err := time.ParseDuration(autotls.CA.CACertificateLifeTime)
	if err != nil {
		return nil, err
	}

	certManager, err := ca.NewCertificateManager(
		config.ctx,
		config.Client,
		maxCertificateLifeTime,
		caCertificateLifeTime,
		autotls.CA.AutoGenerate,
		autotls.CA.Secret,
		autotls.AdditionalTrustRoots,
	)
	if err != nil {
		return nil, err
	}

	return &AutoTlsBackend{
		client:                 config.Client,
		podInfo:                config.PodInfo,
		volumeContext:          config.VolumeContext,
		maxCertificateLifeTime: maxCertificateLifeTime,
		ca:                     autotls.CA,

		certManager: certManager,
	}, nil
}

// use AutoTlsCertLifetime and AutoTlsCertJitterFactor to calculate the certificate lifetime
func (a *AutoTlsBackend) getCertLife() (time.Duration, error) {
	now := time.Now()

	certLife := a.volumeContext.AutoTlsCertLifetime
	if certLife == 0 {
		logger.V(1).Info("certificate lifetime is not set, using default certificate lifetime", "defaultCertLifeTime", DefaultCertLifeTime)
		certLife = DefaultCertLifeTime
	}
	restarterBuffer := a.volumeContext.AutoTlsCertRestartBuffer
	if restarterBuffer == 0 {
		logger.V(1).Info("certificate restart buffer is not set, using default certificate restart buffer", "defaultCertBuffer", DefaultCertBuffer)
		restarterBuffer = DefaultCertBuffer
	}

	if certLife > a.maxCertificateLifeTime {
		logger.V(1).Info("certificate lifetime is greater than the maximum certificate lifetime, using the maximum certificate lifetime",
			"certLife", certLife,
			"maxCertificateLifeTime", a.maxCertificateLifeTime,
		)
		certLife = a.maxCertificateLifeTime
	}

	jitterFactor := a.volumeContext.AutoTlsCertJitterFactor

	jitterFactorAllowedRange := 0.0 < jitterFactor && jitterFactor < 1.0
	if !jitterFactorAllowedRange {
		logger.V(1).Info("invalid jitter factor, using default value", "jitterFactor", jitterFactor)
		jitterFactor = DefaultCertJitter
	}

	randomJitterFactor := rand.Float64() * jitterFactor
	jitterLife := time.Duration(float64(certLife) * jitterFactor)
	jitteredCertLife := certLife - jitterLife

	logger.V(1).Info("jittered certificate lifetime",
		"certLife", certLife,
		"jitteredCertLife", jitteredCertLife,
		"jitterLife", jitterLife,
		"jitterFactor", jitterFactor,
		"randomJitterFactor", randomJitterFactor,
	)

	notAfter := now.Add(jitteredCertLife)
	podExpires := notAfter.Add(-restarterBuffer)
	if podExpires.Before(now) {
		return 0, fmt.Errorf("certificate lifetime is too short, pod will restart before certificate expiration. "+
			"'Now': %v, 'Expires': %v, 'Restart': %v", now, notAfter, podExpires,
		)
	}

	return certLife, nil
}

func (a *AutoTlsBackend) certificateFormat() volume.SecretFormat {
	return a.volumeContext.Format
}

// Convert the certificate to the format required by the volume
// If the format is PKCS12, the certificate will be converted to PKCS12 format,
// otherwise it will be converted to PEM format.
func (a *AutoTlsBackend) certificateConvert(ctx context.Context, cert *ca.Certificate) (map[string]string, error) {
	format := a.certificateFormat()

	trustAnchors, err := a.certManager.GetTrustAnchors(ctx)
	if err != nil {
		return nil, err
	}

	if format == volume.SecretFormatTLSP12 {
		logger.V(1).Info("Converting certificate to PKCS12 format")
		password := a.volumeContext.TlsPKCS12Password

		caCerts := make([]*x509.Certificate, 0, len(trustAnchors))
		for _, caCert := range trustAnchors {
			caCerts = append(caCerts, caCert.Certificate)
		}

		truststore, err := cert.TrustStoreP12(password, caCerts)
		if err != nil {
			return nil, err
		}
		keyStore, err := cert.KeyStoreP12(password, caCerts)
		if err != nil {
			return nil, err
		}
		return map[string]string{
			KeystoreP12FileName:   string(keyStore),
			TruststoreP12FileName: string(truststore),
		}, nil
	}

	pemCACerts := make([]string, 0, len(trustAnchors))

	for _, caCert := range trustAnchors {
		pemCACerts = append(pemCACerts, string(caCert.CertificatePEM()))
	}

	logger.V(1).Info("converting certificate to PEM format")
	return map[string]string{
		PEMTlsCertFileName: string(cert.CertificatePEM()),
		PEMTlsKeyFileName:  string(cert.PrivateKeyPEM()),
		PEMCaCertFileName:  strings.Join(pemCACerts, "\n"),
	}, nil
}

func (k *AutoTlsBackend) GetQualifiedNodeNames(ctx context.Context) ([]string, error) {
	// Default implementation, return nil
	return nil, nil
}

func (a *AutoTlsBackend) GetSecretData(ctx context.Context) (*util.SecretContent, error) {
	addresses, err := a.getAddresses(ctx)
	if err != nil {
		return nil, err
	}

	certLife, err := a.getCertLife()
	if err != nil {
		return nil, err
	}

	notAfter := time.Now().Add(certLife)

	cert, err := a.certManager.SignServerCertificate(addresses, notAfter)

	if err != nil {
		return nil, err
	}

	logger.V(1).Info("signed certificate", "notAfter", notAfter, "addresses", addresses, "certLife", certLife, "certSerialNumber", cert.SerialNumber())

	data, err := a.certificateConvert(ctx, cert)
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
