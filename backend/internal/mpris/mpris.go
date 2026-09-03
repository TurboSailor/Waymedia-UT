// Package mpris publishes container playback as an MPRIS2 player on the
// session bus.
//
// This is the whole point of the daemon: ayatana-indicator-sound watches the
// org.mpris.MediaPlayer2 bus namespace, reads the DesktopEntry property of
// whatever appears and looks up a desktop file by that id (see
// internal/indicator). A matched player shows up in the sound menu — including
// its phone_greeter profile, which is the media widget on the lock screen.
//
// Two consequences shape the code:
//
//   - the bus name has to stay owned for as long as playback lasts: the phone
//     profiles hide players that are not running, so dropping the name would
//     take the lock screen controls away mid-track.
//   - CanRaise must be true, otherwise the indicator renders the player row
//     disabled.
package mpris

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"

	"waymedia/internal/media"
)

const (
	// BusName is ours alone; the suffix after the well-known prefix is free
	// form and only the DesktopEntry property is used for matching.
	BusName = "org.mpris.MediaPlayer2.waymedia"

	objectPath   = "/org/mpris/MediaPlayer2"
	ifaceRoot    = "org.mpris.MediaPlayer2"
	ifacePlayer  = "org.mpris.MediaPlayer2.Player"
	trackPathFmt = "/org/mpris/MediaPlayer2/waymedia/track/%d"
	// noTrack is the MPRIS placeholder for "no track": it is a valid object
	// path, which an empty string is not, and clients special-case it.
	noTrack = dbus.ObjectPath("/org/mpris/MediaPlayer2/TrackList/NoTrack")
)

// Commander runs one media command in the container. Errors are logged and
// swallowed: MPRIS has no useful error channel for a player that is merely
// out of reach, and returning a D-Bus error makes the indicator drop the row.
type Commander interface {
	Dispatch(ctx context.Context, cmd string) error
}

// Options are the fixed strings a player advertises.
type Options struct {
	// Identity is the MPRIS Identity property. The sound menu shows the name
	// from the desktop file instead, so this only reaches generic clients.
	Identity string
	// DesktopID is the desktop file id (without ".desktop") the indicator
	// resolves the player through.
	DesktopID string
	// ArtURL is published as mpris:artUrl for every track. dumpsys exposes no
	// album art at all, and Lomiri renders the media row without any image
	// when the key is missing, leaving the controls unattributed; the package
	// icon at least names the source of the playback. Empty when the daemon
	// runs outside a click package.
	ArtURL string
}

// Player owns the bus name and the exported object.
type Player struct {
	log  *slog.Logger
	cmd  Commander
	opts Options

	mu      sync.Mutex
	conn    *dbus.Conn
	props   *prop.Properties
	trackNo int
	last    media.State
}

// New connects to the session bus, exports the player object and claims the
// bus name.
func New(log *slog.Logger, cmd Commander, opts Options) (*Player, error) {
	p := &Player{log: log, cmd: cmd, opts: opts}

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("mpris: session bus: %w", err)
	}

	props, err := prop.Export(conn, objectPath, p.spec())
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("mpris: export properties: %w", err)
	}
	if err := conn.Export(rootIface{p}, objectPath, ifaceRoot); err != nil {
		conn.Close()
		return nil, fmt.Errorf("mpris: export %s: %w", ifaceRoot, err)
	}
	if err := conn.Export(playerIface{p}, objectPath, ifacePlayer); err != nil {
		conn.Close()
		return nil, fmt.Errorf("mpris: export %s: %w", ifacePlayer, err)
	}
	node := &introspect.Node{
		Name: objectPath,
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			prop.IntrospectData,
			{Name: ifaceRoot, Methods: introspect.Methods(rootIface{p}), Properties: props.Introspection(ifaceRoot)},
			{Name: ifacePlayer, Methods: introspect.Methods(playerIface{p}), Properties: props.Introspection(ifacePlayer)},
		},
	}
	if err := conn.Export(introspect.NewIntrospectable(node), objectPath, "org.freedesktop.DBus.Introspectable"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("mpris: export introspection: %w", err)
	}

	reply, err := conn.RequestName(BusName, dbus.NameFlagDoNotQueue)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("mpris: request %s: %w", BusName, err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		conn.Close()
		return nil, fmt.Errorf("mpris: %s already owned (reply %d)", BusName, reply)
	}

	p.mu.Lock()
	p.conn, p.props = conn, props
	p.mu.Unlock()
	log.Info("mpris exported", "bus", BusName, "desktop_entry", opts.DesktopID, "art_url", opts.ArtURL)
	return p, nil
}

func (p *Player) spec() prop.Map {
	return prop.Map{
		ifaceRoot: {
			"Identity":            ro(p.opts.Identity),
			"DesktopEntry":        ro(p.opts.DesktopID),
			"CanQuit":             ro(false),
			"CanRaise":            ro(true),
			"HasTrackList":        ro(false),
			"SupportedUriSchemes": ro([]string{}),
			"SupportedMimeTypes":  ro([]string{}),
		},
		ifacePlayer: {
			"PlaybackStatus": ro(string(media.StatusStopped)),
			"LoopStatus":     ro("None"),
			"Rate":           ro(1.0),
			"MinimumRate":    ro(1.0),
			"MaximumRate":    ro(1.0),
			"Shuffle":        ro(false),
			// Volume is Android's business: the host mixer already routes the
			// container's stream, so the property exists only because clients
			// read it unconditionally.
			"Volume":        ro(1.0),
			"Position":      ro(int64(0)),
			"Metadata":      ro(map[string]dbus.Variant{"mpris:trackid": dbus.MakeVariant(noTrack)}),
			"CanGoNext":     ro(false),
			"CanGoPrevious": ro(false),
			"CanPlay":       ro(false),
			"CanPause":      ro(false),
			"CanSeek":       ro(false),
			// CanControl false makes clients grey the whole player out, and it
			// is true regardless of what the current session allows.
			"CanControl": ro(true),
		},
	}
}

func ro(v any) *prop.Prop {
	return &prop.Prop{Value: v, Writable: false, Emit: prop.EmitTrue}
}

// Publish mirrors one poll into the exported properties. Only changed values
// are written; prop.SetMust emits PropertiesChanged for each of them.
func (p *Player) Publish(st media.State) {
	p.mu.Lock()
	props := p.props
	if props == nil {
		p.mu.Unlock()
		return
	}
	prev := p.last
	p.last = st
	trackChanged := prev.Session.Title != st.Session.Title ||
		prev.Session.Artist != st.Session.Artist ||
		prev.Session.Package != st.Session.Package ||
		prev.HasSession != st.HasSession
	if trackChanged {
		p.trackNo++
	}
	trackNo := p.trackNo
	p.mu.Unlock()

	status := media.StatusStopped
	if st.HasSession {
		status = st.Session.Status
	}
	set(props, ifacePlayer, "PlaybackStatus", string(status))
	set(props, ifacePlayer, "Position", st.Session.PositionMS*1000)
	set(props, ifacePlayer, "CanPlay", st.HasSession && st.Session.CanPlay())
	set(props, ifacePlayer, "CanPause", st.HasSession && st.Session.CanPause())
	set(props, ifacePlayer, "CanGoNext", st.HasSession && st.Session.CanNext())
	set(props, ifacePlayer, "CanGoPrevious", st.HasSession && st.Session.CanPrevious())
	if trackChanged {
		set(props, ifacePlayer, "Metadata", p.metadata(st, trackNo))
	}
}

// metadata carries what dumpsys gives us: no track length, and album art only
// as the substitute described in Options.ArtURL.
func (p *Player) metadata(st media.State, trackNo int) map[string]dbus.Variant {
	if !st.HasSession {
		return map[string]dbus.Variant{"mpris:trackid": dbus.MakeVariant(noTrack)}
	}
	m := map[string]dbus.Variant{
		"mpris:trackid": dbus.MakeVariant(dbus.ObjectPath(fmt.Sprintf(trackPathFmt, trackNo))),
	}
	if st.Session.Title != "" {
		m["xesam:title"] = dbus.MakeVariant(st.Session.Title)
	}
	if st.Session.Artist != "" {
		m["xesam:artist"] = dbus.MakeVariant([]string{st.Session.Artist})
	}
	if st.Session.Album != "" {
		m["xesam:album"] = dbus.MakeVariant(st.Session.Album)
	}
	if p.opts.ArtURL != "" {
		m["mpris:artUrl"] = dbus.MakeVariant(p.opts.ArtURL)
	}
	return m
}

func set(props *prop.Properties, iface, name string, v any) {
	if cur := props.GetMust(iface, name); fmt.Sprint(cur) == fmt.Sprint(v) {
		return
	}
	props.SetMust(iface, name, v)
}

// Close drops the bus name and the connection. The indicator removes the
// player row as soon as the name disappears.
func (p *Player) Close() {
	p.mu.Lock()
	conn := p.conn
	p.conn, p.props = nil, nil
	p.mu.Unlock()
	if conn == nil {
		return
	}
	if _, err := conn.ReleaseName(BusName); err != nil {
		p.log.Warn("mpris release name", "err", err)
	}
	conn.Close()
}

// ------------------------------------------------------------- interfaces ---

type rootIface struct{ p *Player }

func (rootIface) Quit() *dbus.Error { return nil }

// Raise is what the indicator calls when the player row is tapped. Bringing
// the Android app forward would need a container activity start, which is not
// this daemon's job; the desktop file's Exec opens the WayMedia UI instead, so
// answering the call successfully is enough.
func (rootIface) Raise() *dbus.Error { return nil }

type playerIface struct{ p *Player }

func (i playerIface) Play() *dbus.Error      { return i.p.dispatch(media.CmdPlay) }
func (i playerIface) Pause() *dbus.Error     { return i.p.dispatch(media.CmdPause) }
func (i playerIface) PlayPause() *dbus.Error { return i.p.dispatch(media.CmdPlayPause) }
func (i playerIface) Stop() *dbus.Error      { return i.p.dispatch(media.CmdStop) }
func (i playerIface) Next() *dbus.Error      { return i.p.dispatch(media.CmdNext) }
func (i playerIface) Previous() *dbus.Error  { return i.p.dispatch(media.CmdPrevious) }

// Seek, SetPosition and OpenUri are deliberately absent: CanSeek is false and
// SupportedUriSchemes is empty, so a client that respects those properties
// never calls them, and a silent no-op would only pretend seeking works.

func (p *Player) dispatch(cmd string) *dbus.Error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := p.cmd.Dispatch(ctx, cmd); err != nil {
		p.log.Warn("mpris dispatch failed", "cmd", cmd, "err", err)
	}
	return nil
}
