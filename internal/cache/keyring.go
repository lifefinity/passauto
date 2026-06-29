package cache

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	serviceName = "passauto"
	ttlDefault  = 1 * time.Hour
	tsSeparator = "|"
)

// Get retrieves a cached derived key for the given profile from the OS keychain.
// Returns nil if not found, expired, or secret service unavailable.
func Get(profile string, ttl time.Duration) ([]byte, error) {
	if !secretServiceAvailable() {
		return nil, nil
	}

	if ttl == 0 {
		ttl = ttlDefault
	}

	val, err := keyring.Get(serviceName, profile)
	if err != nil {
		return nil, nil // not found or unavailable
	}

	parts := strings.SplitN(val, tsSeparator, 2)
	if len(parts) != 2 {
		return nil, nil
	}

	ts, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, nil
	}

	if time.Since(time.Unix(ts, 0)) > ttl {
		_ = keyring.Delete(serviceName, profile)
		return nil, nil
	}

	key, err := hex.DecodeString(parts[1])
	if err != nil {
		return nil, nil
	}

	return key, nil
}

// Set stores a derived key in the OS keychain with a timestamp.
func Set(profile string, key []byte) error {
	if !secretServiceAvailable() {
		return nil
	}
	val := fmt.Sprintf("%d%s%s", time.Now().Unix(), tsSeparator, hex.EncodeToString(key))
	return keyring.Set(serviceName, profile, val)
}

// Delete removes a cached key for a profile.
func Delete(profile string) {
	if !secretServiceAvailable() {
		return
	}
	_ = keyring.Delete(serviceName, profile)
}

// Clear removes all cached keys for known profiles.
func Clear(profiles []string) {
	if !secretServiceAvailable() {
		return
	}
	for _, p := range profiles {
		_ = keyring.Delete(serviceName, p)
	}
}
