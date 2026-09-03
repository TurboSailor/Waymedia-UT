// Package waydroid runs shell commands inside the Waydroid container.
//
// Android's media session service lives entirely inside the container and is
// only reachable through a shell there. The container exposes adbd on its
// bridge network, which is the one path that needs neither root nor the
// waydroid CLI, so a long-lived adbd connection is what the daemon uses; it is
// re-dialled after any failure because the session comes and goes on its own.
package waydroid

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// DefaultAddr is the container's adbd endpoint. Waydroid always puts the
// container on 192.168.240.112 of its own bridge; the address is stable across
// sessions and reboots.
const DefaultAddr = "192.168.240.112:5555"

// Timeouts. A dumpsys round trip takes ~150 ms on a busy phone, so anything
// beyond a couple of seconds means the container is wedged rather than slow.
const (
	dialTimeout = 3 * time.Second
	cmdTimeout  = 6 * time.Second
)

// ErrAuth reports that the container's adbd wants an RSA key. Authorising one
// needs a tap inside Android, so there is nothing the daemon can do about it.
var ErrAuth = errADBAuth

// Container is a shell into the Waydroid container. It is safe for concurrent
// use; commands are serialised over the single adb connection.
type Container struct {
	addr string

	mu   sync.Mutex
	conn *adbConn
}

func NewContainer(addr string) *Container {
	if addr == "" {
		addr = DefaultAddr
	}
	return &Container{addr: addr}
}

func (c *Container) Addr() string { return c.addr }

// Transport describes the connection for the status API.
func (c *Container) Transport() string { return "adb " + c.addr }

// Run executes one shell command inside the container. A failure drops the
// connection so the next call re-dials: adbd closes its end when the session
// restarts, and a stale socket only ever returns errors.
func (c *Container) Run(ctx context.Context, cmd string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		conn, err := adbDial(ctx, c.addr, dialTimeout)
		if err != nil {
			return nil, err
		}
		c.conn = conn
	}
	out, err := c.conn.Shell(ctx, cmd, cmdTimeout)
	if err != nil {
		c.conn.Close()
		c.conn = nil
		if errors.Is(err, errADBAuth) {
			return nil, ErrAuth
		}
		return nil, err
	}
	return out, nil
}

// Close drops the connection. The next Run re-dials.
func (c *Container) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// SessionState is what `waydroid status` reports about the container.
type SessionState string

const (
	SessionRunning SessionState = "RUNNING"
	SessionStopped SessionState = "STOPPED"
	SessionUnknown SessionState = "UNKNOWN"
)

// Status asks the host-side waydroid CLI whether a session is up. It is only
// used to tell "nothing is playing" apart from "Waydroid is not running", so a
// failure degrades to UNKNOWN rather than being an error.
func Status(ctx context.Context) SessionState {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "waydroid", "status").Output()
	if err != nil {
		return SessionUnknown
	}
	for _, ln := range strings.Split(string(out), "\n") {
		key, val, ok := strings.Cut(ln, ":")
		if !ok || strings.TrimSpace(key) != "Session" {
			continue
		}
		switch strings.TrimSpace(val) {
		case "RUNNING":
			return SessionRunning
		case "STOPPED":
			return SessionStopped
		}
	}
	return SessionUnknown
}
