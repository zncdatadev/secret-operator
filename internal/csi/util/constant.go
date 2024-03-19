package util

const (
	// Default values for volume context
	CSI_STORAGE_PVC_NAME                        string = "csi.storage.k8s.io/pvc/name"
	CSI_STORAGE_PVC_NAMESPACE                   string = "csi.storage.k8s.io/pvc/namespace"
	CSI_STORAGE_POD_NAME                        string = "csi.storage.k8s.io/pod/name"
	CSI_STORAGE_POD_NAMESPACE                   string = "csi.storage.k8s.io/pod/namespace"
	CSI_STORAGE_POD_UID                         string = "csi.storage.k8s.io/pod/uid"
	CSI_STORAGE_SERVICE_ACCOUNT_NAME            string = "csi.storage.k8s.io/serviceAccount.name"
	CSI_STORAGE_EPHEMERAL                       string = "csi.storage.k8s.io/ephemeral"
	STORAGE_KUBERNETES_CSI_PROVISIONER_IDENTITY string = "storage.kubernetes.io/csiProvisionerIdentity"
	VOLUME_KUBERNETES_STORAGE_PROVISIONER       string = "volume.kubernetes.io/storage-provisioner"

	// User defined annotations for PVC
	SECRETS_ZNCDATA_CLASS string = "secrets.zncdata.dev/class"
	SECRETS_ZNCDATA_SCOPE string = "secrets.zncdata.dev/scope"
)
