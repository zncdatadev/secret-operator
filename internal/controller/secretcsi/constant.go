package secret_csi_plugin

import "path/filepath"

const (
	CSIServiceAccountName     = "secerts-csi-zncdatadev"
	CSIClusterRoleName        = "secrets-csi-zncdatadev"
	CSIClusterRoleBindingName = "secrets-csi-zncdatadev"
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
