package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type MailStorage struct {
	BaseDir string
}

func NewMailStorage(configDir string) *MailStorage {
	baseDir := filepath.Join(configDir, "mails")
	os.MkdirAll(baseDir, 0700)
	return &MailStorage{BaseDir: baseDir}
}

func (s *MailStorage) GetFolderDir(email string, folder string) string {
	accountHash := sha256.Sum256([]byte(email))
	accountDir := filepath.Join(s.BaseDir, hex.EncodeToString(accountHash[:]))
	
	folderHash := sha256.Sum256([]byte(strings.ToUpper(folder)))
	folderDir := filepath.Join(accountDir, hex.EncodeToString(folderHash[:]))
	
	os.MkdirAll(folderDir, 0700)
	return folderDir
}

func (s *MailStorage) SaveMail(email string, folder string, uid uint32, encryptedData []byte) error {
	dir := s.GetFolderDir(email, folder)
	filename := filepath.Join(dir, fmt.Sprintf("%d.enc", uid))
	return os.WriteFile(filename, encryptedData, 0600)
}

func (s *MailStorage) LoadMail(email string, folder string, uid uint32) ([]byte, error) {
	dir := s.GetFolderDir(email, folder)
	filename := filepath.Join(dir, fmt.Sprintf("%d.enc", uid))
	return os.ReadFile(filename)
}

func (s *MailStorage) DeleteMail(email string, folder string, uid uint32) error {
	dir := s.GetFolderDir(email, folder)
	filename := filepath.Join(dir, fmt.Sprintf("%d.enc", uid))
	return os.Remove(filename)
}

func (s *MailStorage) ListUIDs(email string, folder string) ([]uint32, error) {
	dir := s.GetFolderDir(email, folder)
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
