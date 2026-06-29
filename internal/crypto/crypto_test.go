package crypto

import (
	"path/filepath"
	"runtime"
	"testing"
)

func testdataPath(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", name)
}

func TestDeriveKey_Deterministic(t *testing.T) {
	keyPath := testdataPath("test_key_ed25519")

	key1, err := DeriveKey(keyPath, nil)
	if err != nil {
		t.Fatalf("DeriveKey failed: %v", err)
	}
	if len(key1) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key1))
	}

	key2, err := DeriveKey(keyPath, nil)
	if err != nil {
		t.Fatalf("DeriveKey second call failed: %v", err)
	}

	if string(key1) != string(key2) {
		t.Fatal("DeriveKey not deterministic: two calls produced different keys")
	}
}

func TestDeriveKey_InvalidPath(t *testing.T) {
	_, err := DeriveKey("/nonexistent/path", nil)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("my-secret-password")

	ciphertext, err := Encrypt(key, plaintext, []byte("test"))
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if string(ciphertext) == string(plaintext) {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted, err := Decrypt(key, ciphertext, []byte("test"))
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 0xFF

	ciphertext, err := Encrypt(key1, []byte("secret"), []byte("test"))
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(key2, ciphertext, []byte("test"))
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestEncrypt_DifferentNonce(t *testing.T) {
	key := make([]byte, 32)
	plaintext := []byte("same-input")

	c1, _ := Encrypt(key, plaintext, []byte("test"))
	c2, _ := Encrypt(key, plaintext, []byte("test"))

	if string(c1) == string(c2) {
		t.Fatal("two encryptions of same plaintext should produce different ciphertext (random nonce)")
	}
}

func TestGenerateKey_And_DeriveKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "test_key")

	if err := GenerateKey(keyPath, nil); err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	// Should be able to derive a key from the generated file
	key, err := DeriveKey(keyPath, nil)
	if err != nil {
		t.Fatalf("DeriveKey on generated key failed: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}

	// Public key file should exist
	if _, err := filepath.Glob(keyPath + ".pub"); err != nil {
		t.Fatalf("public key file not found: %v", err)
	}
}

func TestGenerateKey_WithPassphrase(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "test_key_protected")
	passphrase := []byte("test-password-123")

	if err := GenerateKey(keyPath, passphrase); err != nil {
		t.Fatalf("GenerateKey with passphrase failed: %v", err)
	}

	// Without passphrase should fail
	_, err := DeriveKey(keyPath, nil)
	if err == nil {
		t.Fatal("expected error without passphrase")
	}

	// With correct passphrase should succeed
	key, err := DeriveKey(keyPath, passphrase)
	if err != nil {
		t.Fatalf("DeriveKey with passphrase failed: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}
}
