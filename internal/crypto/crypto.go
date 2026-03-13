package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

type Manager struct {
	MasterKey []byte
}

func NewManager(key []byte) *Manager {
	return &Manager{MasterKey: key}
}

func (m *Manager) Encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(m.MasterKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, data, nil), nil
}

func (m *Manager) Decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(m.MasterKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func (m *Manager) EncryptJSON(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return m.Encrypt(data)
}

func (m *Manager) DecryptJSON(data []byte, v interface{}) error {
	decrypted, err := m.Decrypt(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(decrypted, v)
}

func GenerateKey() ([]byte, string) {
	newKey := make([]byte, 32)
	rand.Read(newKey)
	return newKey, base64.StdEncoding.EncodeToString(newKey)
}

// DeriveKey derives a 32-byte key from a password and salt using Argon2id.
func DeriveKey(password string, salt []byte) []byte {
	// Recommended parameters for Argon2id: time=3, memory=64MB, threads=4
	return argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
}
