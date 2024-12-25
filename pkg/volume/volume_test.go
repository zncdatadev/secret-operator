package volume

import (
	"reflect"
	"testing"
	"time"

	"github.com/zncdatadev/operator-go/pkg/constants"
)

func TestSecretVolumeContextToMap(t *testing.T) {

	tests := []struct {
		name string
		a    SecretVolumeContext
		want map[string]string
	}{
		{
			name: "empty",
			a:    SecretVolumeContext{},
			want: map[string]string{},
		},
		{
			name: "full",
			a: SecretVolumeContext{
				Pod:                "my-pod",
				PodNamespace:       "my-namespace",
				PodUID:             "my-uid",
				ServiceAccountName: "my-service-account",
				Ephemeral:          "true",
				Provisioner:        "my-provisioner",
				Class:              "my-class",
				Scope: SecretScope{
					Pod:             ScopePod,
					Node:            ScopeNode,
					Services:        []string{"my-service"},
					ListenerVolumes: []string{"my-listener-volume"},
				},
				Format:                   "tls-pem",
				TlsPKCS12Password:        "my-password",
				KerberosServiceNames:     []string{"realm1", "realm2"},
				AutoTlsCertLifetime:      24 * time.Hour,
				AutoTlsCertJitterFactor:  0.1,
				AutoTlsCertRestartBuffer: 5 * time.Minute,
			},
			want: map[string]string{
				CSIStoragePodName:                               "my-pod",
				CSIStoragePodNamespace:                          "my-namespace",
				CSIStoragePodUid:                                "my-uid",
				CSIStorageServiceAccountName:                    "my-service-account",
				CSIStorageEphemeral:                             "true",
				StorageKubernetesCSIProvisionerIdentity:         "my-provisioner",
				constants.AnnotationSecretsClass:                "my-class",
				constants.AnnotationSecretsScope:                "pod,node,service=my-service,listener-volume=my-listener-volume",
				constants.AnnotationSecretsFormat:               "tls-pem",
				constants.AnnotationSecretsPKCS12Password:       "my-password",
				constants.AnnotationSecretsKerberosServiceNames: "realm1,realm2",
				constants.AnnotationSecretCertLifeTime:          "24h0m0s",
				constants.AnnotationSecretsCertJitterFactor:     "0.100000",
				AnnotationSecretsCertRestartBuffer:              "5m0s",
			},
		},
		{
			name: "part-scope",
			a: SecretVolumeContext{
				Scope: SecretScope{
					Node:            ScopeNode,
					ListenerVolumes: []string{"my-listener-volume"},
				},
			},
			want: map[string]string{
				constants.AnnotationSecretsScope: "node,listener-volume=my-listener-volume",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.a.ToMap()

			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("unexpected result: got %v, want %v", result, tt.want)
			}
		})
	}

}

func TestNewvolumeContextFromMap(t *testing.T) {
	tests := []struct {
		name       string
		parameters map[string]string
		expected   *SecretVolumeContext
	}{
		{
			name:       "empty",
			parameters: map[string]string{},
			expected:   &SecretVolumeContext{},
		},
		{
			name: "full",
			parameters: map[string]string{
				CSIStoragePodName:                               "my-pod",
				CSIStoragePodNamespace:                          "my-namespace",
				CSIStoragePodUid:                                "my-uid",
				CSIStorageServiceAccountName:                    "my-service-account",
				CSIStorageEphemeral:                             "true",
				VolumeKubernetesStorageProvisioner:              "my-provisioner",
				constants.AnnotationSecretsClass:                "my-class",
				constants.AnnotationSecretsScope:                "pod,node,service=my-service,listener-volume=my-listener-volume",
				constants.AnnotationSecretsFormat:               "tls-pem",
				constants.AnnotationSecretsPKCS12Password:       "my-password",
				constants.AnnotationSecretsKerberosServiceNames: "realm1,realm2",
				constants.AnnotationSecretCertLifeTime:          "24h0m0s",
				constants.AnnotationSecretsCertJitterFactor:     "0.100000",
				AnnotationSecretsCertRestartBuffer:              "5m0s",
			},
			expected: &SecretVolumeContext{
				Pod:                "my-pod",
				PodNamespace:       "my-namespace",
				PodUID:             "my-uid",
				ServiceAccountName: "my-service-account",
				Ephemeral:          "true",
				Provisioner:        "my-provisioner",
				Class:              "my-class",
				Scope: SecretScope{
					Pod:             ScopePod,
					Node:            ScopeNode,
					Services:        []string{"my-service"},
					ListenerVolumes: []string{"my-listener-volume"},
				},
				Format:                   "tls-pem",
				TlsPKCS12Password:        "my-password",
				KerberosServiceNames:     []string{"realm1", "realm2"},
				AutoTlsCertLifetime:      24 * time.Hour,
				AutoTlsCertJitterFactor:  0.1,
				AutoTlsCertRestartBuffer: 5 * time.Minute,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewvolumeContextFromMap(tt.parameters)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("unexpected result: got %v, want %v", result, tt.expected)
			}
		})
	}
}
