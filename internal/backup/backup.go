package backup

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"

	"github.com/remmody/VaultixIMQ/internal/crypto"
)

type Bundle struct {
	MasterKey []byte `json:"master_key"`
	VaultJson []byte `json:"vault_json"`
}

func ExportProfile(masterKey []byte, vaultData []byte, password string, outputPath string) error {
	bundle := Bundle{
		MasterKey: masterKey,
		VaultJson: vaultData,
	}

	bundleData, err := json.Marshal(bundle)
	if err != nil {
		return err
	}

	// 2. Prepare encryption
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}

	encryptionKey := crypto.DeriveKey(password, salt)
	manager := crypto.NewManager(encryptionKey)

	encrypted, err := manager.Encrypt(bundleData)
	if err != nil {
		return err
	}

	// 3. Write to file (Salt + EncryptedData)
	finalData := append(salt, encrypted...)
	return os.WriteFile(outputPath, finalData, 0600)
}

func ImportProfile(password string, inputPath string) ([]byte, []byte, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, nil, err
	}

	if len(data) < 16+12 {
		return nil, nil, fmt.Errorf("invalid backup file")
	}

	salt, encrypted := data[:16], data[16:]
	encryptionKey := crypto.DeriveKey(password, salt)
	manager := crypto.NewManager(encryptionKey)

	decrypted, err := manager.Decrypt(encrypted)
	if err != nil {
		return nil, nil, fmt.Errorf("incorrect password or corrupted file")
	}

	var bundle Bundle
	if err := json.Unmarshal(decrypted, &bundle); err != nil {
		return nil, nil, err
	}

	return bundle.MasterKey, bundle.VaultJson, nil
}
