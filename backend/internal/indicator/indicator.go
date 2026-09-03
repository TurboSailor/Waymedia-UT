// Package indicator makes the MPRIS player visible to
// ayatana-indicator-sound.
//
// The indicator does not look at the bus name or Identity: it reads the
// player's DesktopEntry property, appends ".desktop" and asks GLib for a
// desktop file with that id. If the lookup fails it logs "unable to find
// application" and drops the player entirely — so the file below is what
// decides whether the lock screen shows any controls at all. Its Name and Icon
// are also what the sound menu renders for the player row.
//
// The file is written by the daemon rather than shipped as a click hook: the
// click desktop id carries the package version (waymedia.turbosailor_waymedia_
// 0.1.0.desktop), so DesktopEntry would have to change on every update, and
// the hook's desktop file has a relative Exec that only resolves through
// Path=. A generated file with a stable name and an absolute Exec avoids both.
package indicator

import (
	"os"
	"path/filepath"
	"strings"

	"waymedia/internal/clickapp"
)

// DesktopID is the stable id published as MPRIS DesktopEntry.
const DesktopID = "waymedia-waydroid"

// FileName is DesktopID as GLib expects to find it on disk.
const FileName = DesktopID + ".desktop"

// PlayerName is the label the sound menu and the lock screen show for the
// player. It names the source of the playback, not the Android app: the
// indicator caches the desktop file per id, so rewriting the name on every
// track change would not reach the menu anyway.
const PlayerName = "Waydroid"

// Registration is where the desktop file went and what went into it.
type Registration struct {
	Path       string
	Registered bool
	Err        string
}

// Ensure writes the desktop file if it is missing or stale. app supplies the
// icon and the application id used for Exec; a zero App (daemon started
// outside a click package) still produces a valid, resolvable entry.
func Ensure(app clickapp.App) Registration {
	dir := applicationsDir()
	path := filepath.Join(dir, FileName)
	reg := Registration{Path: path}

	want := content(app)
	if cur, err := os.ReadFile(path); err == nil && string(cur) == want {
		reg.Registered = true
		return reg
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		reg.Err = err.Error()
		return reg
	}
	// Written through a temporary file: the indicator may be reading the
	// directory at any moment, and a half-written entry is worse than an old
	// one — GLib caches the failed lookup.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(want), 0o644); err != nil {
		reg.Err = err.Error()
		return reg
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		reg.Err = err.Error()
		return reg
	}
	reg.Registered = true
	return reg
}

func applicationsDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "applications")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/home/phablet"
	}
	return filepath.Join(home, ".local", "share", "applications")
}

func content(app clickapp.App) string {
	var b strings.Builder
	b.WriteString("[Desktop Entry]\n")
	b.WriteString("Name=" + PlayerName + "\n")
	b.WriteString("Comment=Управление медиа Waydroid\n")
	b.WriteString("Type=Application\n")
	b.WriteString("Exec=" + execLine(app) + "\n")
	if icon := app.Icon(); icon != "" {
		b.WriteString("Icon=" + icon + "\n")
	}
	b.WriteString("Terminal=false\n")
	// Kept out of the launcher: Lomiri lists click packages, but any other XDG
	// consumer would otherwise show a second WayMedia entry.
	b.WriteString("NoDisplay=true\n")
	b.WriteString("X-Lomiri-Touch=true\n")
	return b.String()
}

// execLine is what runs when the player row in the sound menu is activated.
// Inside a click package that means launching the WayMedia UI through
// lomiri-app-launch; without one there is nothing sensible to start, and
// /bin/true keeps the entry valid (an unresolvable Exec makes GLib reject the
// file, and with it the player).
func execLine(app clickapp.App) string {
	if id := app.AppID(); id != "" {
		return "lomiri-app-launch " + id
	}
	return "/bin/true"
}
