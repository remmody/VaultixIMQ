package config

import (
	"os"
	"path/filepath"
)

func IsVaultSet(configDir string) bool {
	path := filepath.Join(configDir, "vault.vxc")
	_, err := os.Stat(path)
	return err == nil
}
