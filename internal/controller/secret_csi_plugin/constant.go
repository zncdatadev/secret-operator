package secret_csi_plugin

import "path/filepath"

const (
	CSI_SERVICEACCOUNT_NAME     = "secerts-csi-zncdata-labs"
	CSI_CLUSTERROLE_NAME        = "secrets-csi-zncdata-labs"
	CSI_CLUSTERROLEBINDING_NAME = "secrets-csi-zncdata-labs"
)

var (
	PROJECT_ROOT_DIR = filepath.Join(
		"..",
		"..",
		"..",
	)
	CRD_DIRECTORIES = filepath.Join(
		PROJECT_ROOT_DIR,
		"config",
		"crd",
		"bases",
	)
	LOCAL_BIN = filepath.Join(
		PROJECT_ROOT_DIR,
		"bin",
	)
)
