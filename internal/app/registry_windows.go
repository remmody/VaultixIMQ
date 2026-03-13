//go:build windows
package app

import (
	"golang.org/x/sys/windows/registry"
)

func GetLegacyRegistryKey() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\VaultixIMQ`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return ""
	}
	defer k.Close()
	keyStr, _, err := k.GetStringValue("EncryptionKey")
	if err != nil {
		return ""
	}
	return keyStr
}
