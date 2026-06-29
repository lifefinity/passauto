package crypto

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

// KMSEnvelope holds the encrypted DEK + ciphertext for KMS envelope encryption.
type KMSEnvelope struct {
	EncryptedDEK []byte `json:"encrypted_dek"`
	Ciphertext   []byte `json:"ciphertext"`
}

// KMSEncrypt generates a DEK via KMS, encrypts plaintext with AES-256-GCM, and returns
// the sealed envelope (encrypted DEK + ciphertext).
func KMSEncrypt(ctx context.Context, keyID string, plaintext, aad []byte) ([]byte, error) {
	client, err := newKMSClient(ctx)
	if err != nil {
		return nil, err
	}

	out, err := client.GenerateDataKey(ctx, &kms.GenerateDataKeyInput{
		KeyId:   &keyID,
		KeySpec: "AES_256",
	})
	if err != nil {
		return nil, fmt.Errorf("kms GenerateDataKey: %w", err)
	}

	// Encrypt with the plaintext DEK
	ciphertext, err := Encrypt(out.Plaintext, plaintext, aad)
	if err != nil {
		return nil, err
	}

	// Zero plaintext DEK
	for i := range out.Plaintext {
		out.Plaintext[i] = 0
	}

	envelope := KMSEnvelope{
		EncryptedDEK: out.CiphertextBlob,
		Ciphertext:   ciphertext,
	}
	return json.Marshal(envelope)
}

// KMSDecrypt unwraps the DEK via KMS and decrypts the ciphertext.
func KMSDecrypt(ctx context.Context, sealed, aad []byte) ([]byte, error) {
	var envelope KMSEnvelope
	if err := json.Unmarshal(sealed, &envelope); err != nil {
		return nil, fmt.Errorf("parsing KMS envelope: %w", err)
	}

	client, err := newKMSClient(ctx)
	if err != nil {
		return nil, err
	}

	out, err := client.Decrypt(ctx, &kms.DecryptInput{
		CiphertextBlob: envelope.EncryptedDEK,
	})
	if err != nil {
		return nil, fmt.Errorf("kms Decrypt: %w", err)
	}

	plaintext, err := Decrypt(out.Plaintext, envelope.Ciphertext, aad)

	// Zero plaintext DEK
	for i := range out.Plaintext {
		out.Plaintext[i] = 0
	}

	return plaintext, err
}

func newKMSClient(ctx context.Context) (*kms.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	return kms.NewFromConfig(cfg), nil
}
