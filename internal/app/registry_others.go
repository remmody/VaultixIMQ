//go:build !windows
package app

func GetLegacyRegistryKey() string {
	return ""
}
