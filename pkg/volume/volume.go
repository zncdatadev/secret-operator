package volume

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	logger = ctrl.Log.WithName("volume")
)

const (
	KerberosRealmsSplitter string = ","
)

type SecretFormat string

const (
	SecretFormatTLSPEM   SecretFormat = "tls-pem"
	SecretFormatTLSP12   SecretFormat = "tls-p12"
	SecretFormatKerberos SecretFormat = "kerberos"
)

const (
	// kubernetes and sig defained annotations for PVC
	CSIStoragePodName                       string = "csi.storage.k8s.io/pod.name"
	CSIStoragePodNamespace                  string = "csi.storage.k8s.io/pod.namespace"
	CSIStoragePodUid                        string = "csi.storage.k8s.io/pod.uid"
	CSIStorageServiceAccountName            string = "csi.storage.k8s.io/serviceAccount.name"
	CSIStorageEphemeral                     string = "csi.storage.k8s.io/ephemeral"
	StorageKubernetesCSIProvisionerIdentity string = "storage.kubernetes.io/csiProvisionerIdentity"
	VolumeKubernetesStorageProvisioner      string = "volume.kubernetes.io/storage-provisioner"
)

// Annotation for expiration time of zncdata secret for pod.
// When the secret is created, the expiration time is set to the current time plus the lifetime.
// Then we can clean up the secret after expiration time
const (
	SecretZncdataExpirationTime string = "secrets.zncdata.dev/expirationTime"
)

// Labels for k8s search secret
const (
	SecretsZncdataNodeName string = "secrets.zncdata.dev/node"
	SecretsZncdataPod      string = "secrets.zncdata.dev/pod"
	SecretsZncdataService  string = "secrets.zncdata.dev/service"
)

// Zncdata defined annotations for PVCTemplate.
// Then csi driver can extract annotations from PVC to prepare the secret for pod.
const (
	SecretsZncdataClass string = "secrets.zncdata.dev/class"

	// Scope is the scope of the secret.
	// It can be one of the following values:
	// - pod
	// - node
	// - service
	// - listener-volume
	SecretsZncdataScope string = "secrets.zncdata.dev/scope"

	// Format is mounted format of the secret.
	// It can be one of the following values:
	// - tls-pem  A PEM-encoded TLS certificate, include "tls.crt", "tls.key", "ca.crt".
	// - tls-p12 A PKCS#12 archive, include "keystore.p12", "truststore.p12".
	// - kerberos A Kerberos keytab, include "keytab", "krb5.conf".
	SecretsZncdataFormat string = "secrets.zncdata.dev/format"
	// KerberosRealms is the list of Kerberos realms.
	// It is a comma separated list of Kerberos realms.
	// For example: "realm1,realm2"
	SecretsZncdataKerberosRealms string = "secrets.zncdata.dev/kerberosRealms"
	PKCS12Password               string = "secrets.zncdata.dev/tlsPKCS12Password"
	CertLifeTime                 string = "secrets.zncdata.dev/autoTlsCertLifetime"
	CertJitterFactor             string = "secrets.zncdata.dev/autoTlsCertJitterFactor"
)

type SecretVolumeSelector struct {
	// Default values for volume context
	Pod                string `json:"csi.storage.k8s.io/pod.name"`
	PodNamespace       string `json:"csi.storage.k8s.io/pod.namespace"`
	PodUID             string `json:"csi.storage.k8s.io/pod.uid"`
	ServiceAccountName string `json:"csi.storage.k8s.io/serviceAccount.name"`
	Ephemeral          string `json:"csi.storage.k8s.io/ephemeral"`
	Provisioner        string `json:"storage.kubernetes.io/csiProvisionerIdentity"`

	Class  string       `json:"secrets.zncdata.dev/class"`
	Scope  SecretScope  `json:"secrets.zncdata.dev/scope"`
	Format SecretFormat `json:"secrets.zncdata.dev/format"`

	TlsPKCS12Password       string        `json:"secrets.zncdata.dev/tlsPKCS12Password"`
	KerberosRealms          []string      `json:"secrets.zncdata.dev/kerberosRealms"`
	AutoTlsCertLifetime     time.Duration `json:"secrets.zncdata.dev/autoTlsCertLifetime"`
	AutoTlsCertJitterFactor float64       `json:"secrets.zncdata.dev/autoTlsCertJitterFactor"`
}

type ListScope string

const (
	ScopePod            ListScope = "pod"
	ScopeNode           ListScope = "node"
	ScopeService        string    = "service"
	ScopeListenerVolume string    = "listener-volume"
)

type SecretScope struct {
	// this field is k-k pair, key is pod, value is pod
	Pod ListScope `json:"pod"`
	// this field is k-k pair, key is node, value is node
	Node ListScope `json:"node"`
	// this field is k-v pair, key is service name, value is service type
	Services []string `json:"service"`
	// this field is k-v pair, key is listener volume name, value is listener volume type
	ListenerVolumes []string `json:"listener-volume"`
}

func (v SecretVolumeSelector) ToMap() map[string]string {
	out := make(map[string]string)
	if v.Pod != "" {
		out[CSIStoragePodName] = v.Pod
	}
	if v.PodNamespace != "" {
		out[CSIStoragePodNamespace] = v.PodNamespace
	}
	if v.PodUID != "" {
		out[CSIStoragePodUid] = v.PodUID
	}
	if v.ServiceAccountName != "" {
		out[CSIStorageServiceAccountName] = v.ServiceAccountName
	}
	if v.Ephemeral != "" {
		out[CSIStorageEphemeral] = v.Ephemeral
	}
	if v.Provisioner != "" {
		out[StorageKubernetesCSIProvisionerIdentity] = v.Provisioner
	}
	if v.Class != "" {
		out[SecretsZncdataClass] = v.Class
	}
	if v.encodeScope() != "" {
		out[SecretsZncdataScope] = v.encodeScope()
	}
	if v.Format != "" {
		out[SecretsZncdataFormat] = string(v.Format)
	}
	if len(v.KerberosRealms) > 0 {
		out[SecretsZncdataKerberosRealms] = strings.Join(v.KerberosRealms, KerberosRealmsSplitter)
	}
	if v.TlsPKCS12Password != "" {
		out[PKCS12Password] = v.TlsPKCS12Password
	}
	if v.AutoTlsCertLifetime != 0 {
		out[CertLifeTime] = v.AutoTlsCertLifetime.String()
	}
	if v.AutoTlsCertJitterFactor != 0 {
		out[CertJitterFactor] = fmt.Sprintf("%f", v.AutoTlsCertJitterFactor)
	}
	return out
}

func (v SecretVolumeSelector) encodeScope() string {
	var scopes []string
	if v.Scope.Pod != "" && v.Scope.Pod == ScopePod {
		scopes = append(scopes, string(v.Scope.Pod))
	}
	if v.Scope.Node != "" {
		scopes = append(scopes, string(v.Scope.Node))
	}
	if v.Scope.Services != nil {
		for _, services := range v.Scope.Services {
			scopes = append(scopes, fmt.Sprintf("%s=%s", ScopeService, services))
		}
	}
	if v.Scope.ListenerVolumes != nil {
		for _, listenerVolume := range v.Scope.ListenerVolumes {
			scopes = append(scopes, fmt.Sprintf("%s=%s", ScopeListenerVolume, listenerVolume))
		}
	}
	return strings.Join(scopes, ",")
}

func (v SecretVolumeSelector) decodeScope(scope string) SecretScope {
	secretScope := SecretScope{}

	for _, scopes := range strings.Split(scope, ",") {
		kv := strings.Split(scopes, "=")
		switch kv[0] {
		case string(ScopePod):
			secretScope.Pod = ScopePod
		case string(ScopeNode):
			secretScope.Node = ScopeNode
		case ScopeService:
			secretScope.Services = append(secretScope.Services, kv[1])
		case ScopeListenerVolume:
			secretScope.ListenerVolumes = append(secretScope.ListenerVolumes, kv[1])
		default:
			logger.V(0).Info("Unknown scope, skip it", "scope name", kv[0], "scope value", kv[1])
		}
	}
	return secretScope
}

func NewVolumeSelectorFromMap(parameters map[string]string) (*SecretVolumeSelector, error) {
	v := &SecretVolumeSelector{}
	for key, value := range parameters {
		switch key {
		case CSIStoragePodName:
			v.Pod = value
		case CSIStoragePodNamespace:
			v.PodNamespace = value
		case CSIStoragePodUid:
			v.PodUID = value
		case CSIStorageServiceAccountName:
			v.ServiceAccountName = value
		case CSIStorageEphemeral:
			v.Ephemeral = value
		case StorageKubernetesCSIProvisionerIdentity:
			v.Provisioner = value
		case SecretsZncdataClass:
			v.Class = value
		case SecretsZncdataScope:
			v.Scope = v.decodeScope(value)
		case SecretsZncdataFormat:
			v.Format = SecretFormat(value)
		case SecretsZncdataKerberosRealms:
			v.KerberosRealms = strings.Split(value, KerberosRealmsSplitter)
		case PKCS12Password:
			v.TlsPKCS12Password = value
		case CertLifeTime:
			d, err := time.ParseDuration(value)
			if err != nil {
				return nil, err
			}
			v.AutoTlsCertLifetime = d
		case CertJitterFactor:
			i, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, err
			}
			v.AutoTlsCertJitterFactor = float64(i)
		default:
			logger.V(0).Info("Unknown key, skip it", "key", key, "value", value)
		}
	}
	return v, nil
}
