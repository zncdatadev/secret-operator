package secret_csi_plugin

import "path/filepath"

const (
	CSIServiceAccountName     = "secerts-csi-zncdata-labs"
	CSIClusterRoleName        = "secrets-csi-zncdata-labs"
	CSIClusterRoleBindingName = "secrets-csi-zncdata-labs"
)

var (
	ProjectRootDir = filepath.Join(
		"..",
		"..",
		"..",
	)
	CrdDirectories = filepath.Join(
		ProjectRootDir,
		"config",
		"crd",
		"bases",
	)
	LocalBin = filepath.Join(
		ProjectRootDir,
		"bin",
	)
)
