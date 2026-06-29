//go:build darwin || windows

package cache

// secretServiceAvailable always returns true on macOS (Keychain) and
// Windows (Credential Manager) since those backends don't depend on D-Bus.
func secretServiceAvailable() bool {
	return true
}
