// Package totp implements RFC 6238 TOTP (Time-Based One-Time Password).
package totp

import (
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 -- SHA1 is required by TOTP RFC 6238
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// Generate returns a 6-digit TOTP code for the given base32-encoded secret.
func Generate(secret string) (string, error) {
	return GenerateAt(secret, time.Now())
}

// GenerateAt returns a 6-digit TOTP code for the given time.
func GenerateAt(secret string, t time.Time) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(
		strings.ToUpper(strings.TrimSpace(secret)),
	)
	if err != nil {
		return "", fmt.Errorf("decoding TOTP secret: %w", err)
	}

	counter := uint64(t.Unix()) / 30 // #nosec G115 -- Unix time is always positive for TOTP
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	code := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff

	return fmt.Sprintf("%06d", code%1000000), nil
}
