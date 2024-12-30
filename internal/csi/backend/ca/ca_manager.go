package ca

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	logger  = ctrl.Log.WithName("ca-manager")
	caMutex sync.Mutex
)

type CertificateManager struct {
	client               client.Client
	caCertficateLifetime time.Duration
	auto                 bool
	name, namespace      string

	secret *corev1.Secret
	cas    []*CertificateAuthority
}

// NewCertificateManager creates a new CertificateManager
// Get pem key pairs from a secret.
// If the secret does not exist, and auto is enabled, it will create a new self-signed certificate authority.
// If the secret does not exist, and auto is disabled, return error.
// If the secret exists, get certificate authorities from the secret.
// Now, pem key supports only RSA 256.
func NewCertificateManager(
	client client.Client,
	caCertficateLifetime time.Duration,
	auto bool,
	name, namespace string,
) *CertificateManager {
	obj := &CertificateManager{
		client:               client,
		caCertficateLifetime: caCertficateLifetime,
		auto:                 auto,
		name:                 name,
		namespace:            namespace,

		secret: &corev1.Secret{
			ObjectMeta: ctrl.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		},
		cas: []*CertificateAuthority{},
	}
	return obj
}

func (c *CertificateManager) getSecret(ctx context.Context) error {
	err := c.client.Get(ctx, client.ObjectKey{Namespace: c.namespace, Name: c.name}, c.secret)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		logger.V(1).Info("Could not find secret", "name", c.name, "namespace", c.namespace)
		return nil
	}
	logger.V(5).Info("Found secret", "name", c.name, "namespace", c.namespace)
	return nil
}

func (c *CertificateManager) updateSecret(ctx context.Context, data map[string][]byte) error {
	c.secret.Data = data
	// if server side object has been modified, it will raise a conflict error
	// we should get the latest object and retry from the beginning.
	if err := c.client.Update(ctx, c.secret); err != nil {
		return err
	}
	logger.V(0).Info("Saved certificate authorities PEM key pairs to secret", "name", c.name, "namespace", c.namespace)
	return nil
}

func (c *CertificateManager) secretCreateIfDoesNotExist(ctx context.Context) error {
	if c.secret.UID != "" {
		return nil
	}

	logger.V(1).Info("Could not find secret, create a new secret", "name", c.name, "namespace", c.namespace, "auto", c.auto)
	if err := c.client.Create(ctx, c.secret); err != nil {
		return err
	}

	logger.V(1).Info("created a new secret", "name", c.name, "namespace", c.namespace, "auto", c.auto)
	return nil

}

func (c CertificateManager) getPEMKeyPairsFromSecret(ctx context.Context) ([]PEMkeyPair, error) {
	if err := c.getSecret(ctx); err != nil {
		return nil, err
	}

	var keyPairs []PEMkeyPair
	for certName, cert := range c.secret.Data {
		if strings.HasSuffix(certName, ".crt") {
			privateKeyName := strings.TrimSuffix(certName, ".crt") + ".key"
			if privateKey, ok := c.secret.Data[privateKeyName]; ok {
				keyPairs = append(keyPairs, PEMkeyPair{cert, privateKey})
			}
		}
	}

	logger.V(0).Info("got certificate authorities PEM key pairs from secret", "name", c.name, "namespace", c.namespace, "len", len(keyPairs))
	return keyPairs, nil
}

func (c *CertificateManager) updateCertificateAuthoritiesToSecret(
	ctx context.Context,
	cas []*CertificateAuthority,
) error {
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
func (c *CertificateManager) getCertificateAuthorities(pemKeyPairs []PEMkeyPair) ([]*CertificateAuthority, error) {
	cas := make([]*CertificateAuthority, 0)

	for _, keyPair := range pemKeyPairs {
		ca, err := NewCertificateAuthorityFromData(keyPair.CertPEMBlock, keyPair.KeyPEMBlock)
		if err != nil {
			return nil, err
		}
		if ca.Certificate.NotAfter.Before(time.Now()) {
			logger.V(0).Info("certificate authority is expired, skip it.", "serialNumber", ca.SerialNumber(), "notAfter", ca.Certificate.NotAfter)
			continue
		}
		cas = append(cas, ca)
	}

	if len(cas) == 0 {
		if !c.auto {
			return nil, fmt.Errorf(
				`could not find any certificate authorities from secret: {"name": %s, "namespace": %s}, and auto-generate is %s, please create manually`,
				c.name,
				c.namespace,
				strconv.FormatBool(c.auto),
			)
		}

		logger.V(0).Info("could not find any certificate authorities, created a new self-signed certificate authority", "name", c.name, "namespace", c.namespace, "auto", c.auto)
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
func (c *CertificateManager) createSelfSignedCertificateAuthority() (*CertificateAuthority, error) {
	notAfter := time.Now().Add(c.caCertficateLifetime)
	ca, err := NewSelfSignedCertificateAuthority(notAfter, nil, nil)
	if err != nil {
		return nil, err
	}
	logger.V(0).Info("created new self-signed certificate authority", "serialNumber", ca.SerialNumber(), "notAfter", ca.Certificate.NotAfter)
	return ca, nil
}

// sort ca by ca.Certificate.NotAfter as ascending
func (c *CertificateManager) sort(cas []*CertificateAuthority) {
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
func (c *CertificateManager) rotateCertificateAuthority(cas []*CertificateAuthority) ([]*CertificateAuthority, error) {
	if len(cas) == 0 {
		return nil, errors.New("certificate authorities is empty")
	}

	newestCA := cas[len(cas)-1]

	if time.Now().Add(c.caCertficateLifetime / 2).After(newestCA.Certificate.NotAfter) {
		if c.auto {
			newCA, err := newestCA.Rotate(time.Now().Add(c.caCertficateLifetime))
			if err != nil {
				return nil, err
			}
			logger.V(0).Info("rotated certificate authority, because the old ca is about to expire",
				"serialNumber", newestCA.SerialNumber(),
				"notAfter", newCA.Certificate.NotAfter,
			)
			cas = append(cas, newCA)
		} else {
			logger.V(0).Info("certificate authority is about to expire, but auto-generate is disabled, please rotate manually.",
				"serialNumber", newestCA.SerialNumber(),
				"notAfter", newestCA.Certificate.NotAfter,
			)
		}
	} else {
		logger.V(0).Info("certificate authority is still valid, no need to rotate",
			"serialNumber", newestCA.SerialNumber(),
			"notAfter", newestCA.Certificate.NotAfter,
		)
	}

	return cas, nil
}

func (c *CertificateManager) getAliveCertificateAuthority(atAfter time.Time, cas []*CertificateAuthority) *CertificateAuthority {
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
	logger.V(0).Info("got alive certificate authority", "serialNumber", oldestCA.SerialNumber(), "notAfter", oldestCA.Certificate.NotAfter)

	return oldestCA
}

func (c *CertificateManager) GetCertificateAuthority(ctx context.Context, atAfter time.Time) (*CertificateAuthority, error) {
	// retry to get certificate authority
	// if the secret is modified by other clients, it will raise a conflict error
	// we should get the latest object and retry from the beginning.
	caMutex.Lock()
	defer caMutex.Unlock()
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pemKeyPairs, err := c.getPEMKeyPairsFromSecret(ctx)
		if err != nil {
			return err
		}

		c.cas, err = c.getCertificateAuthorities(pemKeyPairs)
		if err != nil {
			return err
		}

		if len(c.cas) == 0 {
			return errors.New("certificate authorities is empty")
		}

		return c.updateCertificateAuthoritiesToSecret(ctx, c.cas)
	}); err != nil {
		return nil, err
	}
	ca := c.getAliveCertificateAuthority(atAfter, c.cas)
	return ca, nil
}

// GetTrustAnchors returns the all ca certificates
func (c *CertificateManager) GetTrustAnchors() []*Certificate {
	trustAnchors := make([]*Certificate, 0, len(c.cas))
	for _, ca := range c.cas {
		// No not publish the private key to other certificates
		trustAnchors = append(trustAnchors, &Certificate{Certificate: ca.Certificate})
	}
	return trustAnchors
}
