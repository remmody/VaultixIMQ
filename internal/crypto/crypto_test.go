package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key, _ := GenerateKey()
	manager := NewManager(key)

	original := []byte("hello world")
	encrypted, err := manager.Encrypt(original)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	if bytes.Equal(encrypted, original) {
		t.Fatal("encrypted data is same as original")
	}

	decrypted, err := manager.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}

	if !bytes.Equal(decrypted, original) {
		t.Errorf("decrypted data (%s) != original (%s)", decrypted, original)
	}
}

func TestDeriveKey(t *testing.T) {
	password := "securepassword"
	salt := []byte("saltsaltsalt")
	
	key1 := DeriveKey(password, salt)
	key2 := DeriveKey(password, salt)
	
	if !bytes.Equal(key1, key2) {
		t.Fatal("DeriveKey with same inputs produced different keys")
	}
	
	key3 := DeriveKey("different", salt)
	if bytes.Equal(key1, key3) {
		t.Fatal("DeriveKey with different password produced same key")
	}
}

func TestEncryptDecryptJSON(t *testing.T) {
	key, _ := GenerateKey()
	manager := NewManager(key)

	type TestData struct {
		Name  string
		Value int
	}

	original := TestData{Name: "Antigravity", Value: 100}
	encrypted, err := manager.EncryptJSON(original)
	if err != nil {
		t.Fatalf("EncryptJSON failed: %v", err)
	}

	var decrypted TestData
	err = manager.DecryptJSON(encrypted, &decrypted)
	if err != nil {
		t.Fatalf("DecryptJSON failed: %v", err)
	}

	if decrypted.Name != original.Name || decrypted.Value != original.Value {
		t.Errorf("decrypted struct %+v != original %+v", decrypted, original)
	}
}
