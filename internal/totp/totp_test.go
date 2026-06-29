package totp

import (
	"testing"
	"time"
)

func TestGenerate_RFC6238Vector(t *testing.T) {
	// RFC 6238 test vector: secret "12345678901234567890" (ASCII) = base32 "GEZDGNBVGY3TQOJQ..."
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	// Unix time 59 → counter 1
	code, err := GenerateAt(secret, time.Unix(59, 0))
	if err != nil {
		t.Fatal(err)
	}
	if code != "287082" {
		t.Errorf("got %s, want 287082", code)
	}
}

func TestGenerate_InvalidSecret(t *testing.T) {
	_, err := Generate("not-valid-base32!!!")
	if err == nil {
		t.Error("expected error for invalid secret")
	}
}

func TestGenerate_ProducesDigits(t *testing.T) {
	// Common test secret
	secret := "JBSWY3DPEHPK3PXP"
	code, err := Generate(secret)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 6 {
		t.Errorf("expected 6 digits, got %q", code)
	}
}
