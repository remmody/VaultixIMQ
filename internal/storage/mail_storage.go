package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

type MailStorage struct {
	BaseDir string
}

func NewMailStorage(configDir string) *MailStorage {
	baseDir := filepath.Join(configDir, "mails")
	os.MkdirAll(baseDir, 0700)
	return &MailStorage{BaseDir: baseDir}
}

func (s *MailStorage) GetAccountDir(email string) string {
	hash := sha256.Sum256([]byte(email))
	accountDir := filepath.Join(s.BaseDir, hex.EncodeToString(hash[:]))
	os.MkdirAll(accountDir, 0700)
	return accountDir
}

func (s *MailStorage) SaveMail(email string, uid uint32, encryptedData []byte) error {
	dir := s.GetAccountDir(email)
	filename := filepath.Join(dir, fmt.Sprintf("%d.enc", uid))
	return os.WriteFile(filename, encryptedData, 0600)
}

func (s *MailStorage) LoadMail(email string, uid uint32) ([]byte, error) {
	dir := s.GetAccountDir(email)
	filename := filepath.Join(dir, fmt.Sprintf("%d.enc", uid))
	return os.ReadFile(filename)
}

func (s *MailStorage) DeleteMail(email string, uid uint32) error {
	dir := s.GetAccountDir(email)
	filename := filepath.Join(dir, fmt.Sprintf("%d.enc", uid))
	return os.Remove(filename)
}

func (s *MailStorage) ListUIDs(email string) ([]uint32, error) {
	dir := s.GetAccountDir(email)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var uids []uint32
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".enc" {
			var uid uint32
			_, err := fmt.Sscanf(entry.Name(), "%d.enc", &uid)
			if err == nil {
				uids = append(uids, uid)
			}
		}
	}
	return uids, nil
}
