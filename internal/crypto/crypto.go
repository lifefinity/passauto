package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/ssh"
)

var (
	hkdfSalt = []byte("autopass-salt-v1")
	hkdfInfo = []byte("autopass-v1")
)

func DeriveKey(keyFilePath string, passphrase []byte) ([]byte, error) {
	keyData, err := os.ReadFile(keyFilePath) // #nosec G304 -- path is from user config
	if err != nil {
		return nil, fmt.Errorf("reading SSH key: %w", err)
	}

	rawBytes, err := extractPrivateKeyBytes(keyData, passphrase)
	if err != nil {
		return nil, fmt.Errorf("parsing SSH key: %w", err)
	}

	return deriveFromRaw(rawBytes)
}

// DeriveKeyFromBytes derives an AES-256 key from raw key material (e.g., from an external command).
func DeriveKeyFromBytes(raw []byte) ([]byte, error) {
	return deriveFromRaw(raw)
}

func deriveFromRaw(rawBytes []byte) ([]byte, error) {
	hkdfReader := hkdf.New(sha256.New, rawBytes, hkdfSalt, hkdfInfo)
	derivedKey := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, derivedKey); err != nil {
		return nil, fmt.Errorf("HKDF expansion: %w", err)
	}

	return derivedKey, nil
}

func Encrypt(key, plaintext, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, aad)
	return ciphertext, nil
}

func Decrypt(key, ciphertext, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBody := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBody, aad)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}

	return plaintext, nil
}

func extractPrivateKeyBytes(pemData, passphrase []byte) ([]byte, error) {
	var rawKey interface{}
	var err error

	if passphrase != nil {
		rawKey, err = ssh.ParseRawPrivateKeyWithPassphrase(pemData, passphrase)
	} else {
		rawKey, err = ssh.ParseRawPrivateKey(pemData)
	}
	if err != nil {
		return nil, err
	}

	switch k := rawKey.(type) {
	case *ed25519.PrivateKey:
		return []byte(*k), nil
	case ed25519.PrivateKey:
		return []byte(k), nil
	default:
		block, _ := pem.Decode(pemData)
		if block == nil {
			return nil, fmt.Errorf("unsupported key type %T and no PEM block found", rawKey)
		}
		return block.Bytes, nil
	}
}

// GenerateKey creates a new ed25519 private key and writes it in OpenSSH format.
// If passphrase is non-empty, the key is encrypted with it.
func GenerateKey(path string, passphrase []byte) error {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generating ed25519 key: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating key directory: %w", err)
	}

	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return fmt.Errorf("creating signer: %w", err)
	}

	comment := ""
	var pemBlock *pem.Block
	if len(passphrase) > 0 {
		pemBlock, err = ssh.MarshalPrivateKeyWithPassphrase(priv, comment, passphrase)
	} else {
		pemBlock, err = ssh.MarshalPrivateKey(priv, comment)
	}
	if err != nil {
		return fmt.Errorf("marshaling private key: %w", err)
	}

	if err := os.WriteFile(path, pem.EncodeToMemory(pemBlock), 0600); err != nil {
		return fmt.Errorf("writing key file: %w", err)
	}

	// Write public key for reference
	pubKey := ssh.MarshalAuthorizedKey(signer.PublicKey())
	_ = os.WriteFile(path+".pub", pubKey, 0644) // #nosec G306 -- public key

	return nil
}
