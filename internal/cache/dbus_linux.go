//go:build linux || (dragonfly && cgo) || (freebsd && cgo) || netbsd || openbsd

package cache

import (
	"sync"
	"time"

	dbus "github.com/godbus/dbus/v5"
)

const (
	secretServiceName = "org.freedesktop.secrets"
	probeCacheDur     = 30 * time.Second
)

var (
	probeOnce   sync.Mutex
	probeResult bool
	probeTime   time.Time
)

// secretServiceAvailable checks if org.freedesktop.secrets is registered
// on the session bus. The result is cached for 30s to avoid repeated D-Bus
// round-trips on rapid successive calls.
func secretServiceAvailable() bool {
	probeOnce.Lock()
	defer probeOnce.Unlock()

	if !probeTime.IsZero() && time.Since(probeTime) < probeCacheDur {
		return probeResult
	}

	probeResult = probeSecretService()
	probeTime = time.Now()
	return probeResult
}

func probeSecretService() bool {
	conn, err := dbus.SessionBusPrivate()
	if err != nil {
		return false
	}
	if err = conn.Auth(nil); err != nil {
		_ = conn.Close()
		return false
	}
	if err = conn.Hello(); err != nil {
		_ = conn.Close()
		return false
	}
	defer func() { _ = conn.Close() }()

	var hasOwner bool
	err = conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, secretServiceName).Store(&hasOwner)
	if err != nil {
		return false
	}
	return hasOwner
}
