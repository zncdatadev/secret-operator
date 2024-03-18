package util

import "path/filepath"

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
