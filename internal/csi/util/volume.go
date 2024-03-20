package util

// VolumeContextSpec is the struct for create Volume ctx from PVC annotations
type VolumeContextSpec struct {
	// Default values for volume context
	Pod                *string `json:"csi.storage.k8s.io/pod.name"`
	PodNamespace       *string `json:"csi.storage.k8s.io/pod.namespace"`
	PodUID             *string `json:"csi.storage.k8s.io/pod.uid"`
	ServiceAccountName *string `json:"csi.storage.k8s.io/serviceAccount.name"`
	Ephemeral          *string `json:"csi.storage.k8s.io/ephemeral"`
	Provisioner        *string `json:"storage.kubernetes.io/csiProvisionerIdentity"`

	// User defined annotations for PVC
	SecretClassName *string `json:"secrets.zncdata.dev/class"`
	Scope           *string `json:"secrets.zncdata.dev/scope"`
}

// ToMap converts VolumeContextSpec to map
func (v VolumeContextSpec) ToMap() map[string]string {
	m := make(map[string]string)
	if v.Pod != nil {
		m[CSI_STORAGE_POD_NAME] = *v.Pod
	}
	if v.PodNamespace != nil {
		m[CSI_STORAGE_POD_NAMESPACE] = *v.PodNamespace
	}
	if v.PodUID != nil {
		m[CSI_STORAGE_POD_UID] = *v.PodUID
	}
	if v.ServiceAccountName != nil {
		m[CSI_STORAGE_SERVICE_ACCOUNT_NAME] = *v.ServiceAccountName
	}
	if v.Ephemeral != nil {
		m[CSI_STORAGE_EPHEMERAL] = *v.Ephemeral
	}
	if v.Provisioner != nil {
		m[STORAGE_KUBERNETES_CSI_PROVISIONER_IDENTITY] = *v.Provisioner
	}
	if v.SecretClassName != nil {
		m[SECRETS_ZNCDATA_CLASS] = *v.SecretClassName
	}
	if v.Scope != nil {
		m[SECRETS_ZNCDATA_SCOPE] = *v.Scope
	}

	return m
}

func NewVolumeContextFromMap(parameters map[string]string) *VolumeContextSpec {
	v := &VolumeContextSpec{}
	if val, ok := parameters[CSI_STORAGE_POD_NAME]; ok {
		v.Pod = &val
	}
	if val, ok := parameters[CSI_STORAGE_POD_NAMESPACE]; ok {
		v.PodNamespace = &val
	}
	if val, ok := parameters[CSI_STORAGE_POD_UID]; ok {
		v.PodUID = &val
	}
	if val, ok := parameters[CSI_STORAGE_SERVICE_ACCOUNT_NAME]; ok {
		v.ServiceAccountName = &val
	}
	if val, ok := parameters[CSI_STORAGE_EPHEMERAL]; ok {
		v.Ephemeral = &val
	}
	if val, ok := parameters[STORAGE_KUBERNETES_CSI_PROVISIONER_IDENTITY]; ok {
		v.Provisioner = &val
	}
	if val, ok := parameters[SECRETS_ZNCDATA_CLASS]; ok {
		v.SecretClassName = &val
	}
	if val, ok := parameters[SECRETS_ZNCDATA_SCOPE]; ok {
		v.Scope = &val
	}

	return v
}
