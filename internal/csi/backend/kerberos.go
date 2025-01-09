package backend

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	secretsv1alpha1 "github.com/zncdatadev/secret-operator/api/v1alpha1"
	"github.com/zncdatadev/secret-operator/pkg/kerberos"
	"github.com/zncdatadev/secret-operator/pkg/pod_info"
	"github.com/zncdatadev/secret-operator/pkg/util"
	"github.com/zncdatadev/secret-operator/pkg/volume"
)

var _ IBackend = &KerberosBackend{}

type KerberosBackend struct {
	client        client.Client
	podInfo       *pod_info.PodInfo
	volumeContext *volume.SecretVolumeContext
	spec          *secretsv1alpha1.KerberosKeytabSpec
}

func NewKerberosBackend(config *BackendConfig) (IBackend, error) {
	return &KerberosBackend{
		client:        config.Client,
		podInfo:       config.PodInfo,
		volumeContext: config.VolumeContext,
		spec:          config.SecretClass.Spec.Backend.KerberosKeytab,
	}, nil
}

func (k *KerberosBackend) getKrb5Config() *kerberos.Krb5Config {
	return &kerberos.Krb5Config{
		Realm:       k.spec.Realm,
		AdminServer: k.spec.AdminServer.MIT.KadminServer,
		KDC:         k.spec.KDC,
	}
}

func (k *KerberosBackend) GetQualifiedNodeNames(ctx context.Context) ([]string, error) {
	// Default to the node name if no node selector is specified
	return nil, nil
}

// GetSecretData implements Backend.
func (k *KerberosBackend) GetSecretData(ctx context.Context) (*util.SecretContent, error) {
	keytab, err := k.provisionKeytab(ctx)
	if err != nil {
		return nil, err
	}

	krb5Config := k.getKrb5Config().Content()

	return &util.SecretContent{Data: map[string]string{"keytab": string(keytab), "krb5.conf": krb5Config}}, nil
}

func (k *KerberosBackend) provisionKeytab(ctx context.Context) ([]byte, error) {
	adminKeytab, err := k.getAdminKeytab(ctx)
	if err != nil {
		return nil, err
	}
	kadmin := kerberos.NewKadmin(k.getKrb5Config(), &k.spec.AdminPrincipal, adminKeytab)

	principals, err := k.getPrincipals(ctx)
	if err != nil {
		return nil, err
	}

	for _, principal := range principals {
		if err := kadmin.AddPrincipal(principal); err != nil {
			logger.Error(err, "failed to add principal", "principal", principal, "kdc", k.spec.KDC)
			return nil, err
		}
	}

	keytab, err := kadmin.Ktadd(principals...)
	if err != nil {
		logger.Error(err, "failed to provision keytab", "principals", principals)
		return nil, err
	}

	return keytab, nil
}

func (k *KerberosBackend) getAdminKeytab(ctx context.Context) ([]byte, error) {
	obj := &corev1.Secret{}
	if err := k.client.Get(ctx, client.ObjectKey{
		Namespace: k.spec.AdminKeytabSecret.Namespace,
		Name:      k.spec.AdminKeytabSecret.Name,
	}, obj); err != nil {
		return nil, err
	}
	data := obj.Data["keytab"]
	if data == nil {
		return nil, fmt.Errorf("could not find keytab data in secret with name %s in namespace %s", obj.Name, obj.Namespace)
	}
	logger.V(1).Info("get admin keytab from secret", "name", obj.Name, "namespace", obj.Namespace)
	return data, nil
}

func (k *KerberosBackend) getPrincipals(ctx context.Context) ([]string, error) {
	scopedAddresses, err := k.podInfo.GetScopedAddresses(ctx)
	if err != nil {
		return nil, err
	}

	svcNames := k.volumeContext.KerberosServiceNames

	principals := make([]string, 0)

	for _, svcName := range svcNames {
		for _, addr := range scopedAddresses {
			hostname := addr.Hostname
			// only support FQDN
			if hostname != "" {
				principal := svcName + "/" + hostname + "@" + k.spec.Realm
				principals = append(principals, principal)
				logger.V(1).Info("add principal", "principal", principal)
			}
		}
	}

	if len(principals) == 0 {
		return nil, fmt.Errorf("no principals found for service names %v", svcNames)
	}

	return principals, nil

}
