package csi

const (
	SECRET_CLASS_ANNOTATION_NAME         = "secrets.zncdata.dev/class"
	SECRET_SCOPE_ANNOTATION_NAME         = "secrets.zncdata.dev/scope"
	SECRET_POD_NAME_ANNOTATION_NAME      = "csi.storage.k8s.io/pod/name"
	SECRET_POD_NAMESPACE_ANNOTATION_NAME = "csi.storage.k8s.io/pod/namespace"
	SECRET_PVC_NAME_ANNOTATION_NAME      = "csi.storage.k8s.io/pvc/name"
	SECRET_PVC_NAMESPACE_ANNOTATION_NAME = "csi.storage.k8s.io/pvc/namespace"
)
