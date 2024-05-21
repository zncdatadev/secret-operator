package ca

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/zncdatadev/secret-operator/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	logger = ctrl.Log.WithName("ca-manager")
)

var (
	ErrCACertificateNotFound = errors.New("CA certificate not found")
	ErrCAPrivateKeyNotFound  = errors.New("CA private key not found")
)

type CertificateManager struct {
	client                 client.Client
	caCertficateLifetime   time.Duration
	auto                   bool
	name, namespace        string
	certificateAuthorities []*CertificateAuthority
}

// NewCertificateManager creates a new CertificateManager
// Get pem key pairs from a secret.
// If the secret does not exist, and auto is enabled, it will create a new self-signed certificate authority.
// If the secret does not exist, and auto is disabled, return error.
// If the secret exists, get certificate authorities from the secret.
// Now, pem key supports only RSA 256.
func NewCertificateManager(
	ctx context.Context,
	client client.Client,
	caCertficateLifetime time.Duration,
	auto bool,
	name, namespace string,
) (*CertificateManager, error) {
	obj := &CertificateManager{
		client:               client,
		caCertficateLifetime: caCertficateLifetime,
		auto:                 auto,
		name:                 name,
		namespace:            namespace,
	}

	pemKeyPairs, err := obj.getSecret(ctx)

	if err != nil {
		return nil, err
	}

	cas, err := obj.getCertificateAuthorities(ctx, pemKeyPairs)
	if err != nil {
		return nil, err
	}

	obj.certificateAuthorities = cas

	return obj, nil
}

// get pem key pairs from a secret
// if the secret does not exist, return nil.
// when auto is enabled, it will create a new self-signed certificate authority
func (c *CertificateManager) getSecret(ctx context.Context) ([]PEMkeyPair, error) {
	secret := &corev1.Secret{}
	err := c.client.Get(ctx, client.ObjectKey{Namespace: c.namespace, Name: c.name}, secret)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}
		return nil, nil
	}

	var keyPairs []PEMkeyPair

	for certName, cert := range secret.Data {
		if strings.HasSuffix(certName, ".crt") {
			privateKeyName := strings.TrimSuffix(certName, ".crt") + ".key"
			if privateKey, ok := secret.Data[privateKeyName]; ok {
				keyPairs = append(keyPairs, PEMkeyPair{cert, privateKey})
			}
		}
	}

	return keyPairs, nil
}

// save pem key pairs to a secret
// If secret does not exist, create a new secret,
// else update the secret when auto is enabled.
func (c *CertificateManager) savePEMKeyPairsToSecret(ctx context.Context, data map[string][]byte) error {
	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.name,
			Namespace: c.namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "secret-operator",
			},
		},
		Data: data,
	}

	if mutant, err := resource.CreateOrUpdate(ctx, c.client, obj); err != nil {
		return err
	} else if mutant {
		logger.V(0).Info("Saved certificate authorities PEM key pairs to secret", "name", c.name, "namespace", c.namespace)
	}

	return nil
}

func (c *CertificateManager) saveCertificateAuthorities(
	ctx context.Context,
	cas []*CertificateAuthority,
) error {

	if !c.auto {
		return errors.New("auto is disabled, should not save certificate authorities, this will overwrite the existing certificate authorities")
	}

	data := map[string][]byte{}
	for _, ca := range cas {
		fmttedSerialNumber := formatSerialNumber(ca.Certificate.SerialNumber)
		data[fmttedSerialNumber+".crt"] = ca.CertificatePEM()
		data[fmttedSerialNumber+".key"] = ca.privateKeyPEM()
	}

	return c.savePEMKeyPairsToSecret(ctx, data)
}

// Get certificate authorities from a secret, if the secret does not exist,
// create a new self-signed certificate authority.
//
// If auto is disabled and no certificate authority is found, return error.
// Before returning the result, it will check whether the existing certificate is about to expire,
// the check condition is whether it has exceeded half of the certificate's validity period.
// If it exceeds half of the validity period, in the case of auto being true,
// it will automatically create a new certificate, but it will not delete the old certificate.
//
// The new certificate is created based on the old certificate. To ensure the integrity of the certificate chain.
// When checking the certificate, if an existing certificate is found and the certificate is about to expire,
// in the case of auto being false, it will be prompted in the form of a log and will not affect
// the issuance of service certificates.
func (c *CertificateManager) getCertificateAuthorities(ctx context.Context, pemKeyPairs []PEMkeyPair) ([]*CertificateAuthority, error) {
	var cas []*CertificateAuthority

	for _, keyPair := range pemKeyPairs {
		ca, err := NewCertificateAuthorityFromData(keyPair.CertPEMBlock, keyPair.KeyPEMBlock)
		if err != nil {
			return nil, err
		}
		if ca.Certificate.NotAfter.Before(time.Now()) {
			logger.V(0).Info("Certificate authority is expired, skip it.", "serialNumber", ca.SerialNumber(), "notAfter", ca.Certificate.NotAfter)
			continue
		}
		cas = append(cas, ca)
	}

	logger.V(0).Info("Found vaild certificate authorities", "count", len(cas))

	if len(cas) == 0 {
		if !c.auto {
			logger.V(0).Info("Could not find any certificate authorities, and auto-generate is disabled, please create manually")
			return nil, ErrCACertificateNotFound
		}

		logger.V(1).Info("Could not find any certificate authorities, created a new self-signed certificate authority")
		ca, err := c.createSelfSignedCertificateAuthority(c.caCertficateLifetime)
		if err != nil {
			return nil, err
		}

		logger.V(0).Info("Could not find any certificate authorities, created a new self-signed certificate authority",
			"serialNumber", ca.SerialNumber(),
			"notAfter", ca.Certificate.NotAfter,
		)
		cas = append(cas, ca)
	}

	// rotate certificate authority
	cas, err := c.rotateCertificateAuthority(cas)

	if err != nil {
		return nil, err
	}

	// save certificate authorities
	if err := c.saveCertificateAuthorities(ctx, cas); err != nil {
		return nil, err
	}

	return cas, nil
}

// create a new self-signed certificate authority
func (c *CertificateManager) createSelfSignedCertificateAuthority(
	caCertficateLifetime time.Duration,
) (*CertificateAuthority, error) {
	notAfter := time.Now().Add(caCertficateLifetime)
	ca, err := NewSelfSignedCertificateAuthority(notAfter, nil, nil)
	if err != nil {
		return nil, err
	}
	logger.V(0).Info("Created new self-signed certificate authority", "notAfter", ca.Certificate.NotAfter)
	return ca, nil
}

func (c *CertificateManager) rotateCertificateAuthority(
	cas []*CertificateAuthority,
) ([]*CertificateAuthority, error) {

	if len(cas) == 0 {
		return nil, ErrCACertificateNotFound
	}

	var newestCA *CertificateAuthority
	for _, ca := range cas {
		if newestCA == nil || ca.Certificate.NotAfter.After(newestCA.Certificate.NotAfter) {
			newestCA = ca
		}
	}

	if time.Now().Add(c.caCertficateLifetime / 2).After(newestCA.Certificate.NotAfter) {
		if c.auto {
			newCA, err := newestCA.Rotate(time.Now().Add(c.caCertficateLifetime))
			if err != nil {
				return nil, err
			}
			logger.V(0).Info("Rotated certificate authority, because the old ca is about to expire",
				"serialNumber", newestCA.SerialNumber(),
				"notAfter", newCA.Certificate.NotAfter,
			)
			cas = append(cas, newCA)
		} else {
			logger.V(0).Info("Certificate authority is about to expire, but auto-generate is disabled, please rotate manually.",
				"serialNumber", newestCA.SerialNumber(),
				"notAfter", newestCA.Certificate.NotAfter,
			)
		}
	} else {
		logger.V(0).Info("Certificate authority is still valid, no need to rotate",
			"serialNumber", newestCA.SerialNumber(),
			"notAfter", newestCA.Certificate.NotAfter,
		)
	}

	return cas, nil
}

func (c *CertificateManager) GetCertificateAuthority(
	atAfter time.Time,
) (*CertificateAuthority, error) {

	cas := c.certificateAuthorities

	if len(cas) == 0 {
		return nil, ErrCACertificateNotFound
	}

	filtedCAs := []*CertificateAuthority{}

	for _, ca := range cas {
		if ca.Certificate.NotAfter.After(atAfter) {
			filtedCAs = append(filtedCAs, ca)
		} else {
			logger.V(0).Info("Certificate authority expired time before max certificate expired time in secret class configed",
				"serialNumber", ca.SerialNumber(), "notAfter", ca.Certificate.NotAfter,
			)
		}
	}

	// oldese certificate authority
	certificateAuthority := filtedCAs[0]

	for _, ca := range filtedCAs {
		if ca.Certificate.NotAfter.Before(certificateAuthority.Certificate.NotAfter) {
			certificateAuthority = ca
		}
	}
	logger.V(5).Info("Get certificate authority to issue cert", "serialNumber", certificateAuthority.SerialNumber(), "notAfter", certificateAuthority.Certificate.NotAfter)

	return certificateAuthority, nil
}
