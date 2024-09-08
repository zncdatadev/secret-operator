package volume

import (
	"reflect"
	"testing"
	"time"

	"github.com/zncdatadev/operator-go/pkg/constants"
)

func TestSecretVolumeSelectorToMap(t *testing.T) {

	tests := []struct {
		name string
		a    SecretVolumeSelector
		want map[string]string
	}{
		{
			name: "empty",
			a:    SecretVolumeSelector{},
			want: map[string]string{},
		},
		{
			name: "full",
			a: SecretVolumeSelector{
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
				Format:               "tls-pem",
				TlsPKCS12Password:    "my-password",
				KerberosServiceNames: []string{"realm1", "realm2"},
				AutoTlsCertLifetime:  24 * time.Hour,
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
				constants.AnnotationSecretsKerberosServiceNames: "realm1,realm2",
			},
		},
		{
			name: "part-scope",
			a: SecretVolumeSelector{
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

func TestNewVolumeSelectorFromMap(t *testing.T) {
	tests := []struct {
		name       string
		parameters map[string]string
		expected   *SecretVolumeSelector
	}{
		{
			name:       "empty",
			parameters: map[string]string{},
			expected:   &SecretVolumeSelector{},
		},
		{
			name: "full",
			parameters: map[string]string{
				CSIStoragePodName:                               "my-pod",
				CSIStoragePodNamespace:                          "my-namespace",
				CSIStoragePodUid:                                "my-uid",
				CSIStorageServiceAccountName:                    "my-service-account",
				CSIStorageEphemeral:                             "true",
				StorageKubernetesCSIProvisionerIdentity:         "my-provisioner",
				constants.AnnotationSecretsClass:                "my-class",
				constants.AnnotationSecretsScope:                "pod,node,service=my-service,listener-volume=my-listener-volume",
				constants.AnnotationSecretsFormat:               "tls-pem",
				constants.AnnotationSecretsKerberosServiceNames: "realm1,realm2",
			},
			expected: &SecretVolumeSelector{
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
				Format:               "tls-pem",
				KerberosServiceNames: []string{"realm1", "realm2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewVolumeSelectorFromMap(tt.parameters)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("unexpected result: got %v, want %v", result, tt.expected)
			}
		})
	}
}
