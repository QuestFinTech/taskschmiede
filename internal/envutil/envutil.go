// Package envutil provides environment variable expansion for configuration files.
package envutil

import (
	"os"
	"regexp"
)

// envVarRegex matches ${VAR} patterns for environment variable expansion.
var envVarRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

// ExpandEnvVars replaces ${VAR} patterns in s with the corresponding
// environment variable values. Variables that are not set (or empty) are
// left unchanged so that optional placeholders survive.
func ExpandEnvVars(s string) string {
	return envVarRegex.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[2 : len(match)-1]
		if value := os.Getenv(varName); value != "" {
			return value
		}
		return match
	})
}
