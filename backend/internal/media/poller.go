package media

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"waymedia/internal/waydroid"
)

// Android media keys, as `cmd media_session dispatch` spells them.
const (
	CmdPlay      = "play"
	CmdPause     = "pause"
	CmdPlayPause = "play-pause"
	CmdNext      = "next"
	CmdPrevious  = "previous"
	CmdStop      = "stop"
)

// ErrUnknownCommand is returned for anything not in the list above; the HTTP
// API turns it into a 400 rather than passing junk into a container shell.
var ErrUnknownCommand = errors.New("media: unknown command")

const (
	dumpCmd = "dumpsys media_session"
	// statusEvery bounds how often the host-side `waydroid status` runs while
	// the container is unreachable: it forks a python CLI, which is far more
	// expensive than the poll itself, and only serves to tell "session down"
	// apart from "session up but adbd wedged".
	statusEvery = 15 * time.Second
)

// State is everything the daemon knows about container playback after a poll.
type State struct {
	Session    Session
	HasSession bool
	// AppName is a human readable name for Session.Package. Android does not
	// expose localised labels over a shell, so this is either a package's
	// non-localised label or a prettified package name.
	AppName string

	Reachable      bool
	ContainerState waydroid.SessionState
	Err            string
	LastPoll       time.Time
}

// Intervals are the two poll rates: playback needs a fresh position and title,
// idling does not.
type Intervals struct {
	Active time.Duration
	Idle   time.Duration
}

// Poller keeps State in sync with the container.
type Poller struct {
	c    *waydroid.Container
	log  *slog.Logger
	iv   func() Intervals
	emit func(State)

	kick chan struct{}

	mu     sync.Mutex
	state  State
	labels map[string]string
	probed map[string]bool

	lastStatusAt time.Time
	lastStatus   waydroid.SessionState
}

func NewPoller(c *waydroid.Container, log *slog.Logger, iv func() Intervals, onChange func(State)) *Poller {
	if onChange == nil {
		onChange = func(State) {}
	}
	return &Poller{
		c:      c,
		log:    log,
		iv:     iv,
		emit:   onChange,
		kick:   make(chan struct{}, 1),
		labels: map[string]string{},
		probed: map[string]bool{},
		state:  State{Session: Session{Status: StatusStopped}, ContainerState: waydroid.SessionUnknown},
	}
}

func (p *Poller) State() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// Kick forces an out-of-band poll. Used right after a dispatch so the UI and
// MPRIS see the new status without waiting out the interval.
func (p *Poller) Kick() {
	select {
	case p.kick <- struct{}{}:
	default:
	}
}

// Run polls until ctx is cancelled, which is the only clean way out.
func (p *Poller) Run(ctx context.Context) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.kick:
		case <-timer.C:
		}
		p.poll(ctx)
		if !timer.Stop() {
			// Drain a timer that fired while the poll was running, otherwise
			// the next select returns immediately and the poll rate collapses.
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(p.nextDelay())
	}
}

func (p *Poller) nextDelay() time.Duration {
	iv := p.iv()
	st := p.State()
	if st.Reachable && st.HasSession && st.Session.Status == StatusPlaying {
		return iv.Active
	}
	return iv.Idle
}

// Dispatch sends one media key into the container. Android routes it to the
// session that owns the media button, which is the session the daemon
// publishes.
func (p *Poller) Dispatch(ctx context.Context, cmd string) error {
	switch cmd {
	case CmdPlay, CmdPause, CmdPlayPause, CmdNext, CmdPrevious, CmdStop:
	default:
		return fmt.Errorf("%w: %q", ErrUnknownCommand, cmd)
	}
	key, send := effectiveKey(cmd, p.State())
	if !send {
		return nil
	}
	if _, err := p.c.Run(ctx, "cmd media_session dispatch "+key); err != nil {
		return err
	}
	// Android applies the key asynchronously; a poll right away would still
	// report the old status, so give the session a moment to settle.
	time.AfterFunc(250*time.Millisecond, p.Kick)
	return nil
}

// effectiveKey adapts a command to what the current session actually accepts.
//
// Players built on androidx.media3 commonly advertise ACTION_PLAY_PAUSE
// without ACTION_PLAY or ACTION_PAUSE (Yandex Music reports actions=524283,
// which has PAUSE and PLAY_PAUSE but no PLAY), and a KEYCODE_MEDIA_PLAY they
// never claimed is free to be ignored. Toggling instead is only safe in the
// state where the toggle lands on the requested one, so a redundant request —
// play while playing, pause while paused — is dropped rather than inverted.
func effectiveKey(cmd string, st State) (string, bool) {
	if !st.HasSession {
		return cmd, true
	}
	s := st.Session
	toggleOnly := s.Actions&actionPlayPause != 0
	switch cmd {
	case CmdPlay:
		if s.Actions&actionPlay != 0 || !toggleOnly {
			return CmdPlay, true
		}
		if s.Status == StatusPlaying {
			return "", false
		}
		return CmdPlayPause, true
	case CmdPause:
		if s.Actions&actionPause != 0 || !toggleOnly {
			return CmdPause, true
		}
		if s.Status != StatusPlaying {
			return "", false
		}
		return CmdPlayPause, true
	default:
		return cmd, true
	}
}

func (p *Poller) poll(ctx context.Context) {
	out, err := p.c.Run(ctx, dumpCmd)
	now := time.Now()

	next := State{LastPoll: now}
	if err != nil {
		next.Session = Session{Status: StatusStopped}
		next.Err = err.Error()
		next.ContainerState = p.containerState(ctx, now, true)
		p.publish(next)
		return
	}

	dump := ParseDump(string(out))
	sess, ok := dump.Current()
	next.Reachable = true
	next.ContainerState = waydroid.SessionRunning
	next.Session = sess
	next.HasSession = ok
	if ok {
		next.AppName = p.appName(ctx, sess.Package)
	}
	p.mu.Lock()
	p.lastStatus, p.lastStatusAt = waydroid.SessionRunning, now
	p.mu.Unlock()
	p.publish(next)
}

// containerState answers "is Waydroid even running" without asking on every
// poll. A reachable adbd already proves the session is up.
func (p *Poller) containerState(ctx context.Context, now time.Time, refresh bool) waydroid.SessionState {
	p.mu.Lock()
	last, at := p.lastStatus, p.lastStatusAt
	p.mu.Unlock()

	if !refresh || (last != waydroid.SessionUnknown && now.Sub(at) < statusEvery) {
		return last
	}
	st := waydroid.Status(ctx)
	p.mu.Lock()
	p.lastStatus, p.lastStatusAt = st, now
	p.mu.Unlock()
	return st
}

// publish stores the new state and notifies the subscriber only when something
// it can observe changed: MPRIS PropertiesChanged on every poll would wake the
// indicator (and the shell) once a second for nothing.
func (p *Poller) publish(next State) {
	p.mu.Lock()
	prev := p.state
	p.state = next
	p.mu.Unlock()

	if significantChange(prev, next) {
		p.emit(next)
	}
}

func significantChange(a, b State) bool {
	if a.HasSession != b.HasSession || a.Reachable != b.Reachable ||
		a.ContainerState != b.ContainerState || a.Err != b.Err || a.AppName != b.AppName {
		return true
	}
	x, y := a.Session, b.Session
	return x.Status != y.Status || x.Title != y.Title || x.Artist != y.Artist ||
		x.Album != y.Album || x.Package != y.Package || x.Actions != y.Actions
}

// ------------------------------------------------------------- app labels ---

// appName resolves a display name for a package once and caches it. Localised
// labels live in the APK's resources and are unreachable from a shell, so a
// package without a non-localised label keeps a prettified name.
func (p *Poller) appName(ctx context.Context, pkg string) string {
	if pkg == "" {
		return ""
	}
	p.mu.Lock()
	name, cached := p.labels[pkg]
	probed := p.probed[pkg]
	p.mu.Unlock()
	if cached {
		return name
	}
	fallback := prettyPkg(pkg)
	if probed {
		return fallback
	}

	p.mu.Lock()
	p.probed[pkg] = true
	p.mu.Unlock()

	out, err := p.c.Run(ctx, "dumpsys package "+pkg)
	if err != nil {
		return fallback
	}
	label := parseNonLocalizedLabel(string(out))
	if label == "" {
		return fallback
	}
	p.mu.Lock()
	p.labels[pkg] = label
	p.mu.Unlock()
	return label
}

func parseNonLocalizedLabel(out string) string {
	for _, ln := range strings.Split(out, "\n") {
		idx := strings.Index(ln, "nonLocalizedLabel=")
		if idx < 0 {
			continue
		}
		v := ln[idx+len("nonLocalizedLabel="):]
		if end := strings.Index(v, " icon="); end >= 0 {
			v = v[:end]
		}
		if v = strings.TrimSpace(v); v != "" && v != "null" {
			return v
		}
	}
	return ""
}

// prettyPkg turns "ru.yandex.music" into "Music" — the best guess available
// without reading the app's resources.
func prettyPkg(pkg string) string {
	seg := pkg
	if i := strings.LastIndexByte(seg, '.'); i >= 0 && i+1 < len(seg) {
		seg = seg[i+1:]
	}
	if seg == "" {
		return pkg
	}
	return strings.ToUpper(seg[:1]) + seg[1:]
}
