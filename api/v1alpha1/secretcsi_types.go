/*
Copyright 2024 zncdata-labs.

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
	CSIPLUGIN_IMAGE_REPOSITORY = "quay.io/zncdata/secret-csi-plugin"
	CSIPLUGIN_IMAGE_TAG        = "v0.0.1"
	CSIPLUGIN_IMAGE_PULLPOLICY = "IfNotPresent"

	NODE_DRIVER_REGISTER_IMAGE_REPOSITORY = "registry.k8s.io/sig-storage/csi-node-driver-registrar"
	NODE_DRIVER_REGISTER_IMAGE_TAG        = "v2.8.0"
	NODE_DRIVER_REGISTER_IMAGE_PULLPOLICY = "IfNotPresent"

	CSI_PROVISIONER_IMAGE_REPOSITORY = "registry.k8s.io/sig-storage/csi-provisioner"
	CSI_PROVISIONER_IMAGE_TAG        = "v3.5.0"
	CSI_PROVISIONER_IMAGE_PULLPOLICY = "IfNotPresent"

	LIVENESS_PROBE_IMAGE_REPOSITORY = "registry.k8s.io/sig-storage/livenessprobe"
	LIVENESS_PROBE_IMAGE_TAG        = "v2.11.0"
	LIVENESS_PROBE_IMAGE_PULLPOLICY = "IfNotPresent"
)

// SecretCSISpec defines the desired state of SecretCSI
type SecretCSISpec struct {
	CSIPlugin          *CSIPluginSpec          `json:"csiPlugin,omitempty"`
	NodeDriverRegister *NodeDriverRegisterSpec `json:"nodeDriverRegister,omitempty"`
	CSIProvisioner     *CSIProvisionerSpec     `json:"csiProvisioner,omitempty"`
	LivenessProbe      *LivenessProbeSpec      `json:"livenessProbe,omitempty"`
}

type CSIPluginSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="quay.io/zncdata/secret-csi-plugin"
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

type NodeDriverRegisterSpec struct {

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
