package util

const (
	// Default values for volume context
	CSI_STORAGE_POD_NAME                        string = "csi.storage.k8s.io/pod.name"
	CSI_STORAGE_POD_NAMESPACE                   string = "csi.storage.k8s.io/pod.namespace"
	CSI_STORAGE_POD_UID                         string = "csi.storage.k8s.io/pod.uid"
	CSI_STORAGE_SERVICE_ACCOUNT_NAME            string = "csi.storage.k8s.io/serviceAccount.name"
	CSI_STORAGE_EPHEMERAL                       string = "csi.storage.k8s.io/ephemeral"
	STORAGE_KUBERNETES_CSI_PROVISIONER_IDENTITY string = "storage.kubernetes.io/csiProvisionerIdentity"
	VOLUME_KUBERNETES_STORAGE_PROVISIONER       string = "volume.kubernetes.io/storage-provisioner"
)

const (
	// User defined annotations for PVC
	SECRETS_ZNCDATA_CLASS   string = "secrets.zncdata.dev/class"
	SECRETS_ZNCDATA_SCOPE   string = "secrets.zncdata.dev/scope"
	SECRETS_ZNCDATA_NODE    string = "secrets.zncdata.dev/node"
	SECRETS_ZNCDATA_POD     string = "secrets.zncdata.dev/pod"
	SECRETS_ZNCDATA_SERVICE string = "secrets.zncdata.dev/service"
)

const (
	RESTARTER_ZNCDATA_EXPIRES_AT string = "restarter.zncdata.dev/expiresAt"
)

const (
	SECRETS_SCOPE_POD     string = "pod"
	SECRETS_SCOPE_NODE    string = "node"
	SECRETS_SCOPE_SERVICE string = "service"
)
