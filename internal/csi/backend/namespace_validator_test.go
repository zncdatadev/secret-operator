package backend

import (
	"testing"

	secretsv1alpha1 "github.com/zncdatadev/secret-operator/api/v1alpha1"
	"github.com/zncdatadev/secret-operator/pkg/volume"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testNamespaceA  = "ns-a"
	testSecretClass = "test-sc"
	testCASecret    = "ca-secret"
	testAppNS       = "app-ns"
	testKeytab      = "keytab"
	testPlatformNS  = "platform-ns"
)

func TestValidateCrossNamespaceReferences_Allowed(t *testing.T) {
	tests := []struct {
		name         string
		podNamespace string
		secretClass  *secretsv1alpha1.SecretClass
		volumeCtx    *volume.SecretVolumeContext
	}{
		{
			name:         "no cross-namespace references in kerberos backend",
			podNamespace: testNamespaceA,
			volumeCtx:    &volume.SecretVolumeContext{PodNamespace: testNamespaceA},
			secretClass: &secretsv1alpha1.SecretClass{
				ObjectMeta: metav1.ObjectMeta{Name: testSecretClass},
				Spec: secretsv1alpha1.SecretClassSpec{
					Backend: &secretsv1alpha1.BackendSpec{
						KerberosKeytab: &secretsv1alpha1.KerberosKeytabSpec{
							AdminKeytabSecret: &secretsv1alpha1.KeytabSecretSpec{
								Name:      "my-keytab",
								Namespace: testNamespaceA,
							},
						},
					},
				},
			},
		},
		{
			name:         "autotls backend with same namespace",
			podNamespace: testNamespaceA,
			volumeCtx:    &volume.SecretVolumeContext{PodNamespace: testNamespaceA},
			secretClass: &secretsv1alpha1.SecretClass{
				ObjectMeta: metav1.ObjectMeta{Name: testSecretClass},
				Spec: secretsv1alpha1.SecretClassSpec{
					Backend: &secretsv1alpha1.BackendSpec{
						AutoTls: &secretsv1alpha1.AutoTlsSpec{
							CA: &secretsv1alpha1.CASpec{
								Secret: &secretsv1alpha1.SecretSpec{
									Name:      testCASecret,
									Namespace: testNamespaceA,
								},
							},
							AdditionalTrustRoots: []secretsv1alpha1.AdditionalTrustRootSpec{
								{
									Secret: &secretsv1alpha1.SecretSpec{
										Name:      "trust-secret",
										Namespace: testNamespaceA,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:         "k8sSearch with Pod mode (no explicit namespace)",
			podNamespace: testNamespaceA,
			volumeCtx:    &volume.SecretVolumeContext{PodNamespace: testNamespaceA},
			secretClass: &secretsv1alpha1.SecretClass{
				ObjectMeta: metav1.ObjectMeta{Name: testSecretClass},
				Spec: secretsv1alpha1.SecretClassSpec{
					Backend: &secretsv1alpha1.BackendSpec{
						K8sSearch: &secretsv1alpha1.K8sSearchSpec{
							SearchNamespace: &secretsv1alpha1.SearchNamespaceSpec{
								Pod: &secretsv1alpha1.PodSpec{},
							},
						},
					},
				},
			},
		},
		{
			name:         "nil backend - no error",
			podNamespace: testNamespaceA,
			volumeCtx:    &volume.SecretVolumeContext{PodNamespace: testNamespaceA},
			secretClass: &secretsv1alpha1.SecretClass{
				ObjectMeta: metav1.ObjectMeta{Name: testSecretClass},
				Spec:       secretsv1alpha1.SecretClassSpec{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCrossNamespaceReferences(tt.secretClass, tt.volumeCtx)
			if err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

func TestValidateCrossNamespaceReferences_Denied(t *testing.T) {
	tests := []struct {
		name          string
		podNamespace  string
		secretClass   *secretsv1alpha1.SecretClass
		volumeCtx     *volume.SecretVolumeContext
		expectedField string
		expectedReqNs string
	}{
		{
			name:         "kerberos backend cross-namespace denied",
			podNamespace: testNamespaceA,
			volumeCtx:    &volume.SecretVolumeContext{PodNamespace: testNamespaceA},
			secretClass: &secretsv1alpha1.SecretClass{
				ObjectMeta: metav1.ObjectMeta{Name: testSecretClass},
				Spec: secretsv1alpha1.SecretClassSpec{
					Backend: &secretsv1alpha1.BackendSpec{
						KerberosKeytab: &secretsv1alpha1.KerberosKeytabSpec{
							AdminKeytabSecret: &secretsv1alpha1.KeytabSecretSpec{
								Name:      "my-keytab",
								Namespace: "ns-b",
							},
						},
					},
				},
			},
			expectedField: "kerberosKeytab.adminKeytabSecret.namespace",
			expectedReqNs: "ns-b",
		},
		{
			name:         "autotls CA secret cross-namespace denied",
			podNamespace: testNamespaceA,
			volumeCtx:    &volume.SecretVolumeContext{PodNamespace: testNamespaceA},
			secretClass: &secretsv1alpha1.SecretClass{
				ObjectMeta: metav1.ObjectMeta{Name: testSecretClass},
				Spec: secretsv1alpha1.SecretClassSpec{
					Backend: &secretsv1alpha1.BackendSpec{
						AutoTls: &secretsv1alpha1.AutoTlsSpec{
							CA: &secretsv1alpha1.CASpec{
								Secret: &secretsv1alpha1.SecretSpec{
									Name:      testCASecret,
									Namespace: "platform",
								},
							},
						},
					},
				},
			},
			expectedField: "autoTls.ca.secret.namespace",
			expectedReqNs: "platform",
		},
		{
			name:         "autotls additional trust root configmap cross-namespace denied",
			podNamespace: testNamespaceA,
			volumeCtx:    &volume.SecretVolumeContext{PodNamespace: testNamespaceA},
			secretClass: &secretsv1alpha1.SecretClass{
				ObjectMeta: metav1.ObjectMeta{Name: testSecretClass},
				Spec: secretsv1alpha1.SecretClassSpec{
					Backend: &secretsv1alpha1.BackendSpec{
						AutoTls: &secretsv1alpha1.AutoTlsSpec{
							CA: &secretsv1alpha1.CASpec{
								Secret: &secretsv1alpha1.SecretSpec{
									Name:      testCASecret,
									Namespace: testNamespaceA,
								},
							},
							AdditionalTrustRoots: []secretsv1alpha1.AdditionalTrustRootSpec{
								{
									ConfigMap: &secretsv1alpha1.ConfigMapSpec{
										Name:      "trust-cm",
										Namespace: "other-ns",
									},
								},
							},
						},
					},
				},
			},
			expectedField: "autoTls.additionalTrustRoots[0].configMap.namespace",
			expectedReqNs: "other-ns",
		},
		{
			name:         "k8sSearch explicit namespace cross-namespace denied",
			podNamespace: testNamespaceA,
			volumeCtx:    &volume.SecretVolumeContext{PodNamespace: testNamespaceA},
			secretClass: &secretsv1alpha1.SecretClass{
				ObjectMeta: metav1.ObjectMeta{Name: testSecretClass},
				Spec: secretsv1alpha1.SecretClassSpec{
					Backend: &secretsv1alpha1.BackendSpec{
						K8sSearch: &secretsv1alpha1.K8sSearchSpec{
							SearchNamespace: &secretsv1alpha1.SearchNamespaceSpec{
								Name: strPtr("ns-c"),
								Pod:  &secretsv1alpha1.PodSpec{},
							},
						},
					},
				},
			},
			expectedField: "k8sSearch.searchNamespace.name",
			expectedReqNs: "ns-c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCrossNamespaceReferences(tt.secretClass, tt.volumeCtx)
			if err == nil {
				t.Fatal("expected NamespaceValidationError, got nil")
			}
			nsErr, ok := err.(*NamespaceValidationError)
			if !ok {
				t.Fatalf("expected NamespaceValidationError, got %T: %v", err, err)
			}
			if nsErr.Field != tt.expectedField {
				t.Errorf("expected field %q, got %q", tt.expectedField, nsErr.Field)
			}
			if nsErr.RequestedNamespace != tt.expectedReqNs {
				t.Errorf("expected requested namespace %q, got %q", tt.expectedReqNs, nsErr.RequestedNamespace)
			}
			if nsErr.PodNamespace != tt.podNamespace {
				t.Errorf("expected pod namespace %q, got %q", tt.podNamespace, nsErr.PodNamespace)
			}
		})
	}
}

func TestValidateCrossNamespaceReferences_AllowedNamespacesAnnotation(t *testing.T) {
	podNs := testAppNS
	volumeCtx := &volume.SecretVolumeContext{PodNamespace: podNs}

	sc := &secretsv1alpha1.SecretClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "shared-sc",
			Annotations: map[string]string{
				AllowedNamespacesAnnotation: "platform-ns, shared-ns",
			},
		},
		Spec: secretsv1alpha1.SecretClassSpec{
			Backend: &secretsv1alpha1.BackendSpec{
				KerberosKeytab: &secretsv1alpha1.KerberosKeytabSpec{
					AdminKeytabSecret: &secretsv1alpha1.KeytabSecretSpec{
						Name:      testKeytab,
						Namespace: testPlatformNS,
					},
				},
			},
		},
	}

	// Should be allowed because platform-ns is in the whitelist
	err := ValidateCrossNamespaceReferences(sc, volumeCtx)
	if err != nil {
		t.Errorf("expected no error with whitelist, got: %v", err)
	}
}

func TestValidateCrossNamespaceReferences_AllowedNamespacesAnnotation_StillDenied(t *testing.T) {
	podNs := testAppNS
	volumeCtx := &volume.SecretVolumeContext{PodNamespace: podNs}

	sc := &secretsv1alpha1.SecretClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "shared-sc",
			Annotations: map[string]string{
				AllowedNamespacesAnnotation: testPlatformNS,
			},
		},
		Spec: secretsv1alpha1.SecretClassSpec{
			Backend: &secretsv1alpha1.BackendSpec{
				KerberosKeytab: &secretsv1alpha1.KerberosKeytabSpec{
					AdminKeytabSecret: &secretsv1alpha1.KeytabSecretSpec{
						Name:      testKeytab,
						Namespace: "attacker-ns",
					},
				},
			},
		},
	}

	// Should be denied because attacker-ns is not in the whitelist
	err := ValidateCrossNamespaceReferences(sc, volumeCtx)
	if err == nil {
		t.Fatal("expected error when namespace not in whitelist")
	}
}

func TestResolveAllowedNamespaces(t *testing.T) {
	tests := []struct {
		name          string
		podNamespace  string
		annotations   map[string]string
		expectedCount int
	}{
		{
			name:          "no annotation - only pod namespace",
			podNamespace:  testNamespaceA,
			annotations:   nil,
			expectedCount: 1,
		},
		{
			name:          "empty annotation - only pod namespace",
			podNamespace:  testNamespaceA,
			annotations:   map[string]string{AllowedNamespacesAnnotation: ""},
			expectedCount: 1,
		},
		{
			name:          "whitelist with multiple namespaces",
			podNamespace:  testNamespaceA,
			annotations:   map[string]string{AllowedNamespacesAnnotation: "ns-b, ns-c, ns-d"},
			expectedCount: 4, // pod ns + 3 whitelisted
		},
		{
			name:          "whitelist with whitespace",
			podNamespace:  testNamespaceA,
			annotations:   map[string]string{AllowedNamespacesAnnotation: " ns-b , ns-c "},
			expectedCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &secretsv1alpha1.SecretClass{
				ObjectMeta: metav1.ObjectMeta{Annotations: tt.annotations},
			}
			allowed := resolveAllowedNamespaces(sc, tt.podNamespace)
			if len(allowed) != tt.expectedCount {
				t.Errorf("expected %d allowed namespaces, got %d: %v", tt.expectedCount, len(allowed), allowed)
			}
			if !allowed[tt.podNamespace] {
				t.Error("pod namespace should always be allowed")
			}
		})
	}
}

func TestBlockedSystemNamespaces(t *testing.T) {
	// Even with an explicit whitelist, system namespaces must always be blocked.
	podNs := testAppNS
	volumeCtx := &volume.SecretVolumeContext{PodNamespace: podNs}

	sc := &secretsv1alpha1.SecretClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dangerous-sc",
			Annotations: map[string]string{
				AllowedNamespacesAnnotation: "kube-system, kube-public, kube-node-lease",
			},
		},
		Spec: secretsv1alpha1.SecretClassSpec{
			Backend: &secretsv1alpha1.BackendSpec{
				KerberosKeytab: &secretsv1alpha1.KerberosKeytabSpec{
					AdminKeytabSecret: &secretsv1alpha1.KeytabSecretSpec{
						Name:      testKeytab,
						Namespace: blockedNamespaceKubeSystem,
					},
				},
			},
		},
	}

	err := ValidateCrossNamespaceReferences(sc, volumeCtx)
	if err == nil {
		t.Fatal("expected error: kube-system must be blocked even when whitelisted")
	}
	nsErr, ok := err.(*NamespaceValidationError)
	if !ok {
		t.Fatalf("expected NamespaceValidationError, got %T: %v", err, err)
	}
	if nsErr.RequestedNamespace != blockedNamespaceKubeSystem {
		t.Errorf("expected requested namespace 'kube-system', got %q", nsErr.RequestedNamespace)
	}
}

func TestBlockedSystemNamespaces_AllOtherWhitelisted(t *testing.T) {
	// System namespace blocked, but normal cross-namespace still works.
	podNs := testAppNS
	volumeCtx := &volume.SecretVolumeContext{PodNamespace: podNs}

	sc := &secretsv1alpha1.SecretClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mixed-sc",
			Annotations: map[string]string{
				AllowedNamespacesAnnotation: "platform-ns, kube-system",
			},
		},
		Spec: secretsv1alpha1.SecretClassSpec{
			Backend: &secretsv1alpha1.BackendSpec{
				KerberosKeytab: &secretsv1alpha1.KerberosKeytabSpec{
					AdminKeytabSecret: &secretsv1alpha1.KeytabSecretSpec{
						Name:      testKeytab,
						Namespace: testPlatformNS,
					},
				},
			},
		},
	}

	// platform-ns should be allowed (it's in whitelist and not blocked)
	err := ValidateCrossNamespaceReferences(sc, volumeCtx)
	if err != nil {
		t.Errorf("expected no error for non-blocked whitelisted namespace, got: %v", err)
	}
}

func TestBlockedSystemNamespaces_KubePublicAndNodeLease(t *testing.T) {
	for _, blockedNs := range []string{"kube-public", "kube-node-lease"} {
		t.Run(blockedNs, func(t *testing.T) {
			podNs := testAppNS
			volumeCtx := &volume.SecretVolumeContext{PodNamespace: podNs}

			sc := &secretsv1alpha1.SecretClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: testSecretClass,
					Annotations: map[string]string{
						AllowedNamespacesAnnotation: blockedNs,
					},
				},
				Spec: secretsv1alpha1.SecretClassSpec{
					Backend: &secretsv1alpha1.BackendSpec{
						KerberosKeytab: &secretsv1alpha1.KerberosKeytabSpec{
							AdminKeytabSecret: &secretsv1alpha1.KeytabSecretSpec{
								Name:      testKeytab,
								Namespace: blockedNs,
							},
						},
					},
				},
			}

			err := ValidateCrossNamespaceReferences(sc, volumeCtx)
			if err == nil {
				t.Errorf("expected error: %s must be blocked", blockedNs)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
