package volume

import (
	"reflect"
	"testing"
	"time"

	"github.com/zncdatadev/operator-go/pkg/constants"
)

const (
	testPod                = "my-pod"
	testNamespace          = "my-namespace"
	testUID                = "my-uid"
	testServiceAccount     = "my-service-account"
	testEphemeral          = "true"
	testProvisioner        = "my-provisioner"
	testClass              = "my-class"
	testListenerVolume     = "my-listener-volume"
	testPassword           = "my-password"
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
				Pod:                testPod,
				PodNamespace:       testNamespace,
				PodUID:             testUID,
				ServiceAccountName: testServiceAccount,
				Ephemeral:          testEphemeral,
				Provisioner:        testProvisioner,
				Class:              testClass,
				Scope: SecretScope{
					Pod:             ScopePod,
					Node:            ScopeNode,
					Services:        []string{"my-service"},
					ListenerVolumes: []string{testListenerVolume},
				},
				Format:                   SecretFormatTLSPEM,
				TlsPKCS12Password:        testPassword,
				KerberosServiceNames:     []string{"realm1", "realm2"},
				AutoTlsCertLifetime:      24 * time.Hour,
				AutoTlsCertJitterFactor:  0.1,
				AutoTlsCertRestartBuffer: 5 * time.Minute,
			},
			want: map[string]string{
				CSIStoragePodName:                               testPod,
				CSIStoragePodNamespace:                          testNamespace,
				CSIStoragePodUid:                                testUID,
				CSIStorageServiceAccountName:                    testServiceAccount,
				CSIStorageEphemeral:                             testEphemeral,
				StorageKubernetesCSIProvisionerIdentity:         testProvisioner,
				constants.AnnotationSecretsClass:                testClass,
				constants.AnnotationSecretsScope:                "pod,node,service=my-service,listener-volume=my-listener-volume",
				constants.AnnotationSecretsFormat:               string(SecretFormatTLSPEM),
				constants.AnnotationSecretsPKCS12Password:       testPassword,
				constants.AnnotationSecretsKerberosServiceNames: "realm1,realm2",
				constants.AnnotationSecretCertLifeTime:          "24h0m0s",
				constants.AnnotationSecretsCertJitterFactor:     "0.100000",
				constants.AnnotationSecretsCertRestartBuffer:    "5m0s",
			},
		},
		{
			name: "part-scope",
			a: SecretVolumeContext{
				Scope: SecretScope{
					Node:            ScopeNode,
					ListenerVolumes: []string{testListenerVolume},
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
				CSIStoragePodName:                               testPod,
				CSIStoragePodNamespace:                          testNamespace,
				CSIStoragePodUid:                                testUID,
				CSIStorageServiceAccountName:                    testServiceAccount,
				CSIStorageEphemeral:                             testEphemeral,
				VolumeKubernetesStorageProvisioner:              testProvisioner,
				constants.AnnotationSecretsClass:                testClass,
				constants.AnnotationSecretsScope:                "pod,node,service=my-service,listener-volume=my-listener-volume",
				constants.AnnotationSecretsFormat:               string(SecretFormatTLSPEM),
				constants.AnnotationSecretsPKCS12Password:       testPassword,
				constants.AnnotationSecretsKerberosServiceNames: "realm1,realm2",
				constants.AnnotationSecretCertLifeTime:          "24h0m0s",
				constants.AnnotationSecretsCertJitterFactor:     "0.100000",
				constants.AnnotationSecretsCertRestartBuffer:    "5m0s",
			},
			expected: &SecretVolumeContext{
				Pod:                testPod,
				PodNamespace:       testNamespace,
				PodUID:             testUID,
				ServiceAccountName: testServiceAccount,
				Ephemeral:          testEphemeral,
				Provisioner:        testProvisioner,
				Class:              testClass,
				Scope: SecretScope{
					Pod:             ScopePod,
					Node:            ScopeNode,
					Services:        []string{"my-service"},
					ListenerVolumes: []string{testListenerVolume},
				},
				Format:                   SecretFormatTLSPEM,
				TlsPKCS12Password:        testPassword,
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
