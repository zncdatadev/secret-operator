package version

import (
	"fmt"
	"runtime"
	"strings"

	"sigs.k8s.io/yaml"
)

// These are set during build time via -ldflags
var (
	BuildVersion = "N/A"
	GitCommit    = "N/A"
	BuildTime    = "N/A"
)

// VersionInfo holds the version information of the driver
type VersionInfo struct {
	DriverName    string `json:"Driver Name"`
	DriverVersion string `json:"Driver Version"`
	GitCommit     string `json:"Git Commit"`
	BuildTime     string `json:"Build Time"`
	GoVersion     string `json:"Go Version"`
	Compiler      string `json:"Compiler"`
	Platform      string `json:"Platform"`
}

// GetVersion returns the version information of the driver
func GetVersion(driverName string) VersionInfo {
	return VersionInfo{
		DriverName:    driverName,
		DriverVersion: BuildVersion,
		GitCommit:     GitCommit,
		BuildTime:     BuildTime,
		GoVersion:     runtime.Version(),
		Compiler:      runtime.Compiler,
		Platform:      fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// GetVersionYAML returns the version information of the driver
// in YAML format
func GetVersionYAML(driverName string) (string, error) {
	info := GetVersion(driverName)
	marshalled, err := yaml.Marshal(&info)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(marshalled)), nil
}
