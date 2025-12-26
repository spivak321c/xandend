package utils

import (
	"strings"

	"github.com/hashicorp/go-version"
)

// VersionConfig holds current version requirements
type VersionConfig struct {
	CurrentStable string
	MinSupported  string
	Deprecated    string
}

var DefaultVersionConfig = VersionConfig{
	CurrentStable: "0.8.0",  // Latest stable version
	MinSupported:  "0.7.3",  // Minimum supported version
	Deprecated:    "0.7.2",  // Versions below this are deprecated
}

// CheckVersionStatus determines if a node version needs upgrading
func CheckVersionStatus(nodeVersion string, config *VersionConfig) (status string, needsUpgrade bool, severity string) {
	if config == nil {
		config = &DefaultVersionConfig
	}

	// Clean version string (remove 'v' prefix if present)
	nodeVersion = strings.TrimPrefix(nodeVersion, "v")
	
	nodeVer, err := version.NewVersion(nodeVersion)
	if err != nil {
		return "unknown", false, "info"
	}
	
	current, _ := version.NewVersion(config.CurrentStable)
	minSupported, _ := version.NewVersion(config.MinSupported)
	deprecated, _ := version.NewVersion(config.Deprecated)
	
	// Check if deprecated (critical)
	if nodeVer.LessThan(deprecated) {
		return "deprecated", true, "critical"
	}
	
	// Check if below minimum supported (warning)
	if nodeVer.LessThan(minSupported) {
		return "outdated", true, "warning"
	}
	
	// Check if not on latest stable (info)
	if nodeVer.LessThan(current) {
		return "outdated", true, "info"
	}
	
	// On latest or newer
	return "current", false, "none"
}

// GetUpgradeMessage returns a human-readable upgrade message
func GetUpgradeMessage(nodeVersion string, config *VersionConfig) string {
	if config == nil {
		config = &DefaultVersionConfig
	}
	
	_, needsUpgrade, severity := CheckVersionStatus(nodeVersion, config)
	
	if !needsUpgrade {
		return ""
	}
	
	switch severity {
	case "critical":
		return "CRITICAL: This version is deprecated and no longer supported. Upgrade to " + config.CurrentStable + " immediately."
	case "warning":
		return "WARNING: This version is outdated. Please upgrade to " + config.CurrentStable + " soon."
	case "info":
		return "INFO: A newer version " + config.CurrentStable + " is available."
	}
	
	return ""
}