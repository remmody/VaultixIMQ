package app

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"fmt"

	"github.com/remmody/VaultixIMQ/internal/crypto"
	"github.com/zalando/go-keyring"
)

func (c *Core) InitEncryption() {
	// 1. Try Keyring (Standard)
	if secret, err := c.GetSecureKey(); err == nil {
		if key, err := base64.StdEncoding.DecodeString(secret); err == nil && len(key) == 32 {
			c.EncryptionKey = key
			// Cleanup legacy file if it exists
			keyPath := filepath.Join(c.ConfigDir, ".key")
			if _, err := os.Stat(keyPath); err == nil {
				os.Remove(keyPath)
			}
			return
		}
	}

	// 2. Fallback to Legacy .key file (Migration)
	keyPath := filepath.Join(c.ConfigDir, ".key")
	if keyData, err := os.ReadFile(keyPath); err == nil {
		if key, err := base64.StdEncoding.DecodeString(string(keyData)); err == nil && len(key) == 32 {
			c.EncryptionKey = key
			c.SetSecureKey(string(keyData))
			os.Remove(keyPath) 
			return
		}
	}

	// 3. Fallback to Registry (Legacy v2.x migration)
	if keyStr := GetLegacyRegistryKey(); keyStr != "" {
		if key, err := base64.StdEncoding.DecodeString(keyStr); err == nil && len(key) == 32 {
			c.EncryptionKey = key
			c.SetSecureKey(keyStr)
			return
		}
	}

	// 4. Final Step: Start from scratch
	newKey, keyStr := crypto.GenerateKey()
	c.EncryptionKey = newKey
	c.SetSecureKey(keyStr)
}

func (c *Core) GetSecureKey() (string, error) {
	service := "VaultixIMQ"
	user := "MasterKey"
	
	val, err := keyring.Get(service, user)
	if err == nil {
		return val, nil
	}

	// Linux fallback: check if secret service is missing
	if runtime.GOOS == "linux" {
		fallbackPath := filepath.Join(c.ConfigDir, ".master.key")
		if data, err := os.ReadFile(fallbackPath); err == nil {
			return string(data), nil
		}
	}
	return "", err
}

func (c *Core) SetSecureKey(key string) error {
	service := "VaultixIMQ"
	user := "MasterKey"

	err := keyring.Set(service, user, key)
	if err == nil {
		return nil
	}

	// Linux fallback: if D-Bus fails, use a hidden file
	if runtime.GOOS == "linux" {
		fmt.Printf("Keyring failed: %v. Falling back to file.\n", err)
		fallbackPath := filepath.Join(c.ConfigDir, ".master.key")
		return os.WriteFile(fallbackPath, []byte(key), 0600)
	}
	return err
}

