/*
Copyright 2024 zncdatadev.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretClassSpec defines the desired state of SecretClass
type SecretClassSpec struct {
	Backend *BackendSpec `json:"backend,omitempty"`
}

type BackendSpec struct {
	// +kubebuilder:validation:Optional
	AutoTls *AutoTlsSpec `json:"autoTls,omitempty"`
	// +kubebuilder:validation:Optional
	K8sSearch *K8sSearchSpec `json:"k8sSearch,omitempty"`
	// +kubebuilder:validation:Optional
	KerberosKeytab *KerberosKeytabSpec `json:"kerberosKeytab,omitempty"`
}

type AutoTlsSpec struct {
	// Reference to a ConfigMap or Secret containing the trust root.
	// When the key suffix is `.crt`, the value is a base64 encoded DER certificate.
	// When the key suffix is `.der`, the value is a binary DER certificate.
	// +kubebuilder:validation:Optional
	AdditionalTrustRoots []AdditionalTrustRootSpec `json:"additionalTrustRoots,omitempty"`

	// Configures the certificate authority used to issue Pod certificates.
	// +kubebuilder:validation:Required
	CA *CASpec `json:"ca"`

	// Use time.ParseDuration to parse the string
	// Default is 360h (15 days)
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="360h"
	MaxCertificateLifeTime string `json:"maxCertificateLifeTime,omitempty"`
}

type AdditionalTrustRootSpec struct {
	// Reference to a ConfigMap containing the trust root.
	// +kubebuilder:validation:Optional
	ConfigMap *ConfigMapSpec `json:"configMap,omitempty"`

	// Reference to a Secret containing the trust root.
	// +kubebuilder:validation:Optional
	Secret *SecretSpec `json:"secret,omitempty"`
}

type ConfigMapSpec struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

type CASpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	AutoGenerate bool `json:"autoGenerate,omitempty"`

	// Use time.ParseDuration to parse the string
	// Default is 8760h (1 year)
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="8760h"
	CACertificateLifeTime string `json:"caCertificateLifeTime,omitempty"`

	// Reference to a Secret where the CA certificate is stored.
	// +kubebuilder:validation:Required
	Secret *SecretSpec `json:"secret"`

	// +kubebuilder:validation:Optional
	KeyGeneration *KeyGenerationSpec `json:"keyGeneration,omitempty"`
}

type KeyGenerationSpec struct {
	// +kubebuilder:validation:Optional
	RSA *RSASpec `json:"rsa,omitempty"`
}

type RSASpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=2048;3072;4096
	Length int `json:"length"`
}

type SecretSpec struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

type K8sSearchSpec struct {
	// One of the `Name` for namespace or `Pod` for the same namespace with pod.
	// +kubebuilder:validation:Required
	SearchNamespace *SearchNamespaceSpec `json:"searchNamespace"`
}

type SearchNamespaceSpec struct {
	Name *string `json:"name,omitempty"`

	Pod *PodSpec `json:"pod,omitempty"`
}

type PodSpec struct {
}

// SecretClassStatus defines the observed state of SecretClass
type SecretClassStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=secretclasses,scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// SecretClass is the Schema for the secretclasses API
type SecretClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretClassSpec   `json:"spec,omitempty"`
	Status SecretClassStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SecretClassList contains a list of SecretClass
type SecretClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecretClass `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecretClass{}, &SecretClassList{})
}
