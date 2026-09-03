// Command waymediad bridges Android playback inside the Waydroid container to
// the Ubuntu Touch sound indicator, so a track can be paused or skipped from
// the lock screen.
//
// Waydroid runs a whole Android userspace whose media sessions never reach
// Lomiri: MPRIS lives on the host session bus, Android's MediaSession lives in
// the container, and nothing joins them. This daemon polls
// `dumpsys media_session` through the container's adbd, republishes what it
// finds as an MPRIS2 player, and turns MPRIS calls back into
// `cmd media_session dispatch` media keys.
//
// Two platform facts shape the process:
//
//   - ayatana-indicator-sound matches players by their MPRIS DesktopEntry
//     property against a desktop file it can resolve; without that file the
//     player is dropped with a log line and never reaches the sound menu or
//     its greeter profile. internal/indicator writes the file.
//   - Lomiri freezes and then reaps a backgrounded application's whole cgroup,
//     so the bridge cannot live inside the UI process: it runs as a systemd
//     *user* unit (see click/waymediad.service), which also brings it back
//     after a reboot without root or a writable rootfs.
package main

import (
	"context"

	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"waymedia/internal/api"
	"waymedia/internal/clickapp"
	"waymedia/internal/config"
	"waymedia/internal/indicator"
	"waymedia/internal/media"
	"waymedia/internal/mpris"
	"waymedia/internal/waydroid"
)

// fallbackVersion is used when the daemon runs outside a click package, where
// there is no manifest to read the real version from.
const fallbackVersion = "dev"

func main() {
	var (
		addr    = flag.String("addr", waydroid.DefaultAddr, "container adbd address")
		stateIn = flag.String("state-dir", "", "settings directory (default $XDG_DATA_HOME/waymedia)")
		verbose = flag.Bool("v", false, "debug logging")
	)
	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	stateDir := *stateIn
	if stateDir == "" {
		stateDir = defaultStateDir()
	}
	if err := run(log, *addr, stateDir); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func defaultStateDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "waymedia")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/home/phablet"
	}
	return filepath.Join(home, ".local", "share", "waymedia")
}

func version(app clickapp.App) string {
	if !app.IsClick() {
		return fallbackVersion
	}
	return app.Version
}

// artURL turns the package icon into the file URI MPRIS clients expect.
func artURL(app clickapp.App) string {
	icon := app.Icon()
	if icon == "" {
		return ""
	}
	return "file://" + icon
}

func run(log *slog.Logger, addr, stateDir string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app := clickapp.Detect()
	settings, err := config.Load(filepath.Join(stateDir, "config.json"))
	if err != nil {
		return err
	}

	// The desktop file is what makes the indicator accept the player, so a
	// failure here is reported through the API instead of being fatal: the
	// HTTP control surface and the UI still work.
	reg := indicator.Ensure(app)
	if reg.Err != "" {
		log.Error("desktop registration failed", "path", reg.Path, "err", reg.Err)
	} else {
		log.Info("desktop registration", "path", reg.Path, "player", indicator.PlayerName,
			"app_id", app.AppID(), "icon", app.Icon())
	}

	container := waydroid.NewContainer(addr)
	defer container.Close()

	b := &bridge{log: log, container: container, reg: reg, base: ctx, artURL: artURL(app)}
	b.poller = media.NewPoller(container, log, func() media.Intervals {
		s := settings.Get()
		return media.Intervals{Active: s.Poll(), Idle: s.IdlePoll()}
	}, b.onState)

	b.Apply(settings.Get())
	defer b.shutdown()

	srv := api.New(log, b, settings, version(app))
	errs := make(chan error, 1)
	go func() { errs <- srv.Serve(ctx) }()

	select {
	case <-ctx.Done():
		// Drain the API result so a listen failure during shutdown is not
		// silently lost.
		select {
		case err := <-errs:
			return err
		case <-time.After(3 * time.Second):
			return nil
		}
	case err := <-errs:
		if err != nil {
			return fmt.Errorf("api: %w", err)
		}
		return nil
	}
}

// bridge owns the two halves that the settings switch on and off: the poller
// that reads the container and the MPRIS player that publishes it.
type bridge struct {
	log       *slog.Logger
	container *waydroid.Container
	poller    *media.Poller
	reg       indicator.Registration
	// artURL is published for every track; see mpris.Options.ArtURL.
	artURL string
	// base is the daemon's lifetime context; the poller's own context is
	// derived from it every time the bridge is switched back on.
	base context.Context

	mu     sync.Mutex
	player *mpris.Player
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (b *bridge) State() media.State { return b.poller.State() }

func (b *bridge) Dispatch(ctx context.Context, cmd string) error {
	return b.poller.Dispatch(ctx, cmd)
}

func (b *bridge) Registration() indicator.Registration { return b.reg }

func (b *bridge) Transport() string { return b.container.Transport() }

func (b *bridge) Exported() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.player != nil
}

// onState is called by the poller whenever something observable changed.
func (b *bridge) onState(st media.State) {
	b.mu.Lock()
	player := b.player
	b.mu.Unlock()
	if player != nil {
		player.Publish(st)
	}
	b.log.Debug("state",
		"reachable", st.Reachable, "session", st.HasSession,
		"status", st.Session.Status, "title", st.Session.Title, "err", st.Err)
}

// Apply brings the bridge in line with the settings: enabled means an owned
// MPRIS name and a running poller, disabled means neither — a disabled bridge
// must stop the adb round trips, which is what the user turned off.
//
// It runs synchronously on the caller (the API handler), so a settings
// response already reports the state the caller just asked for.
func (b *bridge) Apply(s config.Settings) {
	if !s.Enabled {
		b.shutdown()
		return
	}

	b.mu.Lock()
	if b.player == nil {
		player, err := mpris.New(b.log, b.poller, mpris.Options{
			Identity:  indicator.PlayerName,
			DesktopID: indicator.DesktopID,
			ArtURL:    b.artURL,
		})
		if err != nil {
			b.mu.Unlock()
			b.log.Error("mpris export failed", "err", err)
			return
		}
		b.player = player
		// Publish immediately: the indicator reads the properties as soon as
		// the bus name appears, and an empty player would be hidden by the
		// phone profiles until the first poll.
		player.Publish(b.poller.State())
	}
	if b.cancel == nil {
		pollCtx, cancel := context.WithCancel(b.base)
		b.cancel = cancel
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			b.poller.Run(pollCtx)
		}()
	}
	b.mu.Unlock()
	b.poller.Kick()
}

func (b *bridge) shutdown() {
	b.mu.Lock()
	cancel, player := b.cancel, b.player
	b.cancel, b.player = nil, nil
	b.mu.Unlock()

	if cancel != nil {
		cancel()
		b.wg.Wait()
	}
	if player != nil {
		player.Close()
	}
	// The connection is re-dialled on the next poll; holding an idle socket
	// into a container that may restart meanwhile buys nothing.
	b.container.Close()
}
