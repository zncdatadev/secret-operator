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

const (

	// default values
	CSIPluginImageRepository = "quay.io/zncdatadev/secret-csi-driver"
	CSIPluginImageTag        = "v0.0.1"
	CSIPluginImagePullPolicy = "IfNotPresent"

	NodeDriverRegisterImageRepository = "registry.k8s.io/sig-storage/csi-node-driver-registrar"
	NodeDriverRegisterImageTag        = "v2.8.0"
	NodeDriverRegisterImagePullPolicy = "IfNotPresent"

	CSIProvisionerImageRepository = "registry.k8s.io/sig-storage/csi-provisioner"
	CSIProvisionerImageTag        = "v3.5.0"
	CSIProvisionerImagePullPolicy = "IfNotPresent"

	LivenessProbeImageRepository = "registry.k8s.io/sig-storage/livenessprobe"
	LivenessProbeImageTag        = "v2.11.0"
	LivenessProbeImagePullPolicy = "IfNotPresent"
)

// SecretCSISpec defines the desired state of SecretCSI
type SecretCSISpec struct {
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	CSIDriver *CSIDriverSpec `json:"csiDriver,omitempty"`
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	NodeDriverRegistrar *NodeDriverRegistrarSpec `json:"nodeDriverRegistrar,omitempty"`
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	CSIProvisioner *CSIProvisionerSpec `json:"csiProvisioner,omitempty"`
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	LivenessProbe *LivenessProbeSpec `json:"livenessProbe,omitempty"`
}

type CSIDriverSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="quay.io/zncdatadev/secret-csi-driver"
	Repository string `json:"repository,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="v0.0.1"
	Tag string `json:"tag,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="IfNotPresent"
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	PullPolicy string `json:"pullPolicy,omitempty"`

	// +kubebuilder:validation:Optional
	Logging *LoggingSpec `json:"logging,omitempty"`
}

type NodeDriverRegistrarSpec struct {

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="registry.k8s.io/sig-storage/csi-node-driver-registrar"
	Repository string `json:"repository,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="v2.8.0"
	Tag string `json:"tag,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="IfNotPresent"
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	PullPolicy string `json:"pullPolicy,omitempty"`

	// +kubebuilder:validation:Optional
	Logging *LoggingSpec `json:"logging,omitempty"`
}

type CSIProvisionerSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="registry.k8s.io/sig-storage/csi-provisioner"
	Repository string `json:"repository,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="v3.5.0"
	Tag string `json:"tag,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="IfNotPresent"
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	PullPolicy string `json:"pullPolicy,omitempty"`

	// +kubebuilder:validation:Optional
	Logging *LoggingSpec `json:"logging,omitempty"`
}

type LivenessProbeSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="registry.k8s.io/sig-storage/livenessprobe"
	Repository string `json:"repository,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="v2.11.0"
	Tag string `json:"tag,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="IfNotPresent"
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	PullPolicy string `json:"pullPolicy,omitempty"`

	// +kubebuilder:validation:Optional
	Logging *LoggingSpec `json:"logging,omitempty"`
}

type LoggingSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="info"
	Level string `json:"level,omitempty"`
}

// SecretCSIStatus defines the observed state of SecretCSI
type SecretCSIStatus struct {
	Conditions []metav1.Condition `json:"conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SecretCSI is the Schema for the secretcsis API
type SecretCSI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretCSISpec   `json:"spec,omitempty"`
	Status SecretCSIStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SecretCSIList contains a list of SecretCSI
type SecretCSIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecretCSI `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecretCSI{}, &SecretCSIList{})
}
