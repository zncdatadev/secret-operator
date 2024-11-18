package secretcsi

import "path/filepath"

const (
	CSIServiceAccountName     = "secrets-csi-zncdatadev"
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
