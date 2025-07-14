package ca

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	secretsv1alpha1 "github.com/zncdatadev/secret-operator/api/v1alpha1"
	"github.com/zncdatadev/secret-operator/pkg/pod_info"
)

var (
	logger  = ctrl.Log.WithName("ca-manager")
	caMutex sync.Mutex
)

type CertificateManager interface {
	GetTrustAnchors(ctx context.Context) ([]*Certificate, error)
	SignServerCertificate(addresses []pod_info.Address, notAfter time.Time) (*Certificate, error)
	SignClientCertificate(addresses []pod_info.Address, notAfter time.Time) (*Certificate, error)
}

var _ CertificateManager = &certificateManager{}

type certificateManager struct {
	client                 client.Client
	maxCertificateLifeTime time.Duration
	caCertificateLifetime  time.Duration
	auto                   bool
	additionalTrustRoots   []secretsv1alpha1.AdditionalTrustRootSpec
	rsaKeyLength           int

	caSecret *corev1.Secret

	selectedCA *CertificateAuthority
	cas        []*CertificateAuthority
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
	maxCertificateLifeTime time.Duration,
	caCertificateLifetime time.Duration,
	auto bool,
	caSecretSpec *secretsv1alpha1.SecretSpec,
	additionalTrustRoots []secretsv1alpha1.AdditionalTrustRootSpec,
	rsaKeyLength int,

) (CertificateManager, error) {

	caSecret, err := getSecret(ctx, client, caSecretSpec.Name, caSecretSpec.Namespace)
	if err != nil {
		return nil, err
	}

	cm := &certificateManager{
		client:                client,
		caCertificateLifetime: caCertificateLifetime,
		auto:                  auto,
		caSecret:              caSecret,
		cas:                   []*CertificateAuthority{},
		rsaKeyLength:          rsaKeyLength,
	}

	ca, err := cm.getCertificateAuthority(ctx)
	if err != nil {
		return nil, err
	}

	// init ca immediately
	cm.selectedCA = ca

	return cm, nil
}

func (c *certificateManager) updateSecret(ctx context.Context, data map[string][]byte) error {
	c.caSecret.Data = data
	// if server side object has been modified, it will raise a conflict error
	// we should get the latest object and retry from the beginning.
	if err := c.client.Update(ctx, c.caSecret); err != nil {
		return err
	}
	logger.V(1).Info("saved certificate authorities PEM key pairs to secret", "name", c.caSecret.Name, "namespace", c.caSecret.Namespace)
	return nil
}

func (c *certificateManager) secretCreateIfDoesNotExist(ctx context.Context) error {
	if c.caSecret.UID != "" {
		return nil
	}

	logger.V(1).Info("could not find secret, create a new secret", "name", c.caSecret.Name, "namespace", c.caSecret.Namespace, "auto", c.auto)
	if err := c.client.Create(ctx, c.caSecret); err != nil {
		return err
	}

	logger.V(1).Info("created a new secret", "name", c.caSecret.Name, "namespace", c.caSecret.Namespace, "auto", c.auto)
	return nil

}

func (c certificateManager) getPEMKeyPairsFromSecret() []PEMkeyPair {
	var keyPairs []PEMkeyPair

	if len(c.caSecret.Data) == 0 {
		logger.V(1).Info("secret data is nil", "name", c.caSecret.Name, "namespace", c.caSecret.Namespace)
		return keyPairs
	}

	for certName, cert := range c.caSecret.Data {
		if strings.HasSuffix(certName, ".crt") {
			privateKeyName := strings.TrimSuffix(certName, ".crt") + ".key"
			if privateKey, ok := c.caSecret.Data[privateKeyName]; ok {
				keyPairs = append(keyPairs, PEMkeyPair{cert, privateKey})
			}
		}
	}

	logger.V(1).Info("got certificate authorities PEM key pairs from secret", "name", c.caSecret.Name, "namespace", c.caSecret.Namespace, "len", len(keyPairs))
	return keyPairs
}

func (c *certificateManager) updateCertificateAuthoritiesToSecret(ctx context.Context, cas []*CertificateAuthority) error {
	if !c.auto {
		return fmt.Errorf("could not save certificate authorities, because auto is %s, this will overwrite the existing certificate authorities",
			strconv.FormatBool(c.auto),
		)
	}

	if err := c.secretCreateIfDoesNotExist(ctx); err != nil {
		return err
	}

	c.sort(cas)

	data := map[string][]byte{}
	for i, ca := range cas {
		prefix := strconv.Itoa(i)
		data[prefix+".ca.crt"] = ca.CertificatePEM()
		data[prefix+".ca.key"] = ca.privateKeyPEM()
	}

	if err := c.updateSecret(ctx, data); err != nil {
		return err
	}

	return nil
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
func (c *certificateManager) getCertificateAuthorities(pemKeyPairs []PEMkeyPair) ([]*CertificateAuthority, error) {
	cas := make([]*CertificateAuthority, 0)

	for _, keyPair := range pemKeyPairs {
		ca, err := NewCertificateAuthorityFromData(keyPair.CertPEMBlock, keyPair.KeyPEMBlock, c.rsaKeyLength)
		if err != nil {
			return nil, err
		}
		if ca.Certificate.NotAfter.Before(time.Now()) {
			logger.V(1).Info("certificate authority is expired, skip it.", "serialNumber", ca.SerialNumber(), "notAfter", ca.Certificate.NotAfter)
			continue
		}
		cas = append(cas, ca)
	}

	if len(cas) == 0 {
		if !c.auto {
			return nil, fmt.Errorf(
				`could not find any certificate authorities from secret: {"name": %s, "namespace": %s}, and auto-generate is %s, please create manually`,
				c.caSecret.Name,
				c.caSecret.Namespace,
				strconv.FormatBool(c.auto),
			)
		}

		logger.V(1).Info("could not find any valid certificate authorities, created a new self-signed certificate authority",
			"name", c.caSecret.Name, "namespace", c.caSecret.Namespace, "auto", c.auto,
		)
		ca, err := c.createSelfSignedCertificateAuthority()
		if err != nil {
			return nil, err
		}

		cas = append(cas, ca)
	}

	// rotate certificate authority
	cas, err := c.rotateCertificateAuthority(cas)

	if err != nil {
		return nil, err
	}

	return cas, nil
}

// create a new self-signed certificate authority only no certificate authority is found
func (c *certificateManager) createSelfSignedCertificateAuthority() (*CertificateAuthority, error) {
	notAfter := time.Now().Add(c.caCertificateLifetime)
	ca, err := NewSelfSignedCertificateAuthority(notAfter, nil, nil, c.rsaKeyLength)
	if err != nil {
		return nil, err
	}
	logger.V(1).Info("created new self-signed certificate authority", "serialNumber", ca.SerialNumber(), "notAfter", ca.Certificate.NotAfter, "rsaKeyLength", c.rsaKeyLength)
	return ca, nil
}

// sort ca by ca.Certificate.NotAfter as ascending
func (c *certificateManager) sort(cas []*CertificateAuthority) {
	slices.SortFunc(cas, func(i, j *CertificateAuthority) int {
		// i < j return -1
		if i.Certificate.NotAfter.Before(j.Certificate.NotAfter) {
			return -1
		}
		// i > j return 1
		if i.Certificate.NotAfter.After(j.Certificate.NotAfter) {
			return 1
		}
		// i == j
		return 0
	})
}

// rotate certificate authority
// if the certificate authority is about to expire, it will create a new certificate authority
func (c *certificateManager) rotateCertificateAuthority(cas []*CertificateAuthority) ([]*CertificateAuthority, error) {
	if len(cas) == 0 {
		return nil, errors.New("certificate authorities is empty when rotating certificate authority")
	}

	// sort certificate authority as ascending
	c.sort(cas)

	newestCA := cas[len(cas)-1]

	if time.Now().Add(c.caCertificateLifetime / 2).After(newestCA.Certificate.NotAfter) {
		if c.auto {
			newCA, err := newestCA.Rotate(time.Now().Add(c.caCertificateLifetime))
			if err != nil {
				return nil, err
			}
			logger.V(1).Info("rotated certificate authority, because the old ca is about to expire",
				"serialNumber", newestCA.SerialNumber(),
				"notAfter", newCA.Certificate.NotAfter,
			)
			cas = append(cas, newCA)
		} else {
			logger.V(1).Info("certificate authority is about to expire, but auto-generate is disabled, please rotate manually.",
				"serialNumber", newestCA.SerialNumber(),
				"notAfter", newestCA.Certificate.NotAfter,
			)
		}
	} else {
		logger.V(1).Info("certificate authority is still valid, no need to rotate",
			"serialNumber", newestCA.SerialNumber(),
			"notAfter", newestCA.Certificate.NotAfter,
		)
	}

	return cas, nil
}

func (c *certificateManager) getAliveCertificateAuthority(atAfter time.Time, cas []*CertificateAuthority) *CertificateAuthority {
	cas = slices.DeleteFunc(cas, func(ca *CertificateAuthority) bool {
		return ca.Certificate.NotAfter.Before(atAfter)
	})

	oldestCA := slices.MinFunc(cas, func(a, b *CertificateAuthority) int {
		if a.Certificate.NotAfter.Before(b.Certificate.NotAfter) {
			return -1
		}
		if a.Certificate.NotAfter.After(b.Certificate.NotAfter) {
			return 1
		}
		return 0
	})
	logger.V(1).Info("got alive certificate authority", "serialNumber", oldestCA.SerialNumber(), "notAfter", oldestCA.Certificate.NotAfter)

	return oldestCA
}

// Get the certificate authority before the expiration time, the expiration time from the secret class
func (c *certificateManager) getCertificateAuthority(ctx context.Context) (*CertificateAuthority, error) {
	atAfter := time.Now().Add(c.maxCertificateLifeTime) // server cert lifetime in secret class configed
	// retry to get certificate authority
	// if the secret is modified by other clients, it will raise a conflict error
	// we should get the latest object and retry from the beginning.
	caMutex.Lock()
	defer caMutex.Unlock()
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pemKeyPairs := c.getPEMKeyPairsFromSecret()

		cas, err := c.getCertificateAuthorities(pemKeyPairs)
		if err != nil {
			return err
		}

		if len(cas) == 0 {
			return errors.New("certificate authorities is empty")
		}

		c.cas = cas

		return c.updateCertificateAuthoritiesToSecret(ctx, c.cas)
	}); err != nil {
		return nil, err
	}
	ca := c.getAliveCertificateAuthority(atAfter, c.cas)
	return ca, nil
}

func (c *certificateManager) getAdditionalTrustRoots(ctx context.Context) ([]*Certificate, error) {
	additionalTrustRoots := make([]*Certificate, 0, len(c.additionalTrustRoots))

	for _, additionalTrustRoot := range c.additionalTrustRoots {
		configMap, err := getConfigmap(ctx, c.client, additionalTrustRoot.ConfigMap.Name, additionalTrustRoot.ConfigMap.Namespace)
		if err != nil {
			return nil, err
		}
		if configMap == nil {
			return nil, fmt.Errorf("could not find configmap: %s/%s", additionalTrustRoot.ConfigMap.Namespace, additionalTrustRoot.ConfigMap.Name)
		}

		certs, err := c.processConfigmapDataToCert(configMap.Data, configMap.BinaryData)
		if err != nil {
			return nil, err
		}

		additionalTrustRoots = append(additionalTrustRoots, certs...)

		logger.V(1).Info("got additional trust roots from configmap", "name", configMap.Name, "namespace", configMap.Namespace, "len", len(certs))

		secret, err := getSecret(ctx, c.client, additionalTrustRoot.Secret.Name, additionalTrustRoot.Secret.Namespace)
		if err != nil {
			return nil, err
		}
		if secret == nil {
			return nil, fmt.Errorf("could not find secret: %s/%s", additionalTrustRoot.Secret.Namespace, additionalTrustRoot.Secret.Name)
		}

		certs, err = c.processSecretDataToCert(secret.Data)
		if err != nil {
			return nil, err
		}
		additionalTrustRoots = append(additionalTrustRoots, certs...)
		logger.V(1).Info("got additional trust roots from secret", "name", secret.Name, "namespace", secret.Namespace, "len", len(certs))
	}

	logger.V(1).Info("got additional trust roots", "len", len(additionalTrustRoots))

	return additionalTrustRoots, nil
}

func (c *certificateManager) processConfigmapDataToCert(data map[string]string, binaryData map[string][]byte) ([]*Certificate, error) {
	certs := make([]*Certificate, 0, len(data))
	for key, certPEM := range data {
		cert, err := c.convertDataToCert(key, []byte(certPEM))
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}

	for key, certPEM := range binaryData {
		cert, err := c.convertDataToCert(key, certPEM)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}

	return certs, nil
}

func (c *certificateManager) processSecretDataToCert(data map[string][]byte) ([]*Certificate, error) {
	certs := make([]*Certificate, 0, len(data))
	for key, certPEM := range data {
		cert, err := c.convertDataToCert(key, certPEM)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

// convertDataToCert convert data to cert
// if the key is suffix with .crt, it will parse the data with base64 encoded DER certificate
// if the key is suffix with .der, it will parse the data with binary DER certificate
func (c *certificateManager) convertDataToCert(key string, data []byte) (*Certificate, error) {
	if strings.HasSuffix(key, ".crt") {
		// Parse PEM format certificate
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("failed to decode PEM data for key %s", key)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate for key %s: %v", key, err)
		}
		return &Certificate{Certificate: cert}, nil
	} else if strings.HasSuffix(key, ".der") {
		// Parse DER format certificate
		cert, err := x509.ParseCertificate(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse DER certificate for key %s: %v", key, err)
		}
		return &Certificate{Certificate: cert}, nil
	}

	return nil, fmt.Errorf("unsupported certificate format for key %s, must end with .crt or .der", key)
}

// GetTrustAnchors returns the all ca certificates
func (c *certificateManager) GetTrustAnchors(ctx context.Context) ([]*Certificate, error) {
	trustAnchors := make([]*Certificate, 0, len(c.cas)+len(c.additionalTrustRoots))
	for _, ca := range c.cas {
		// Do not publish the private key to the trust anchors
		trustAnchors = append(trustAnchors, &Certificate{Certificate: ca.Certificate})
	}

	additionalTrustRoots, err := c.getAdditionalTrustRoots(ctx)

	if err != nil {
		logger.Error(err, "failed to get additional trust roots")
		return nil, err
	}

	trustAnchors = append(trustAnchors, additionalTrustRoots...)

	return trustAnchors, nil
}

func (c *certificateManager) SignServerCertificate(addresses []pod_info.Address, notAfter time.Time) (*Certificate, error) {
	cert, err := c.selectedCA.SignServerCertificate(addresses, notAfter)
	if err != nil {
		return nil, err
	}

	return cert, nil
}

func (c *certificateManager) SignClientCertificate(addresses []pod_info.Address, notAfter time.Time) (*Certificate, error) {
	cert, err := c.selectedCA.SignClientCertificate(addresses, notAfter)
	if err != nil {
		return nil, err
	}

	return cert, nil
}

func getSecret(ctx context.Context, cli client.Client, name, namespace string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := cli.Get(ctx, client.ObjectKeyFromObject(secret), secret)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}
		logger.V(1).Info("could not find secret, will create a new secret", "name", name, "namespace", namespace)
		return secret, nil
	}
	logger.V(5).Info("found secret", "name", name, "namespace", namespace)
	return secret, nil
}

func getConfigmap(ctx context.Context, cli client.Client, name, namespace string) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	err := cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, configMap)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}
		logger.V(1).Info("could not find configmap", "name", name, "namespace", namespace)
		return nil, nil
	}
	logger.V(5).Info("found configmap", "name", name, "namespace", namespace)
	return configMap, nil
}
