// Package clickapp describes the click package the daemon was installed from.
//
// The daemon needs three things out of it: the version to report over the API,
// the application id to launch the UI with (lomiri-app-launch expects
// <package>_<hook>_<version>), and the icon path for the generated desktop
// file. None of them are compiled in, because the package version changes on
// every update while the running binary does not.
package clickapp

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// App is a resolved click installation. A zero App means "not running from a
// click package" — a build tree, a manual copy — and every accessor degrades
// to empty.
type App struct {
	// Dir is the unpack directory the binary lives in, e.g.
	// /opt/click.ubuntu.com/waymedia.turbosailor/0.1.0.
	Dir string
	// StableDir is Dir addressed through the "current" symlink when that
	// symlink points at Dir. Paths handed to other processes use it, so they
	// keep resolving after an update replaces the version directory.
	StableDir string

	Name    string
	Version string
	// Hook is the manifest hook carrying the desktop file, i.e. the middle
	// part of the application id.
	Hook string
}

// Detect resolves the package around the running executable. The binary is
// installed as <Dir>/bin/waymediad.
func Detect() App {
	exe, err := os.Executable()
	if err != nil {
		return App{}
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(filepath.Dir(exe))

	name, version, hook, ok := readManifest(dir)
	if !ok {
		return App{}
	}
	return App{
		Dir:       dir,
		StableDir: stableDir(dir),
		Name:      name,
		Version:   version,
		Hook:      hook,
	}
}

// IsClick reports whether the daemon runs from an installed package.
func (a App) IsClick() bool { return a.Dir != "" && a.Name != "" && a.Version != "" }

// AppID is what lomiri-app-launch takes.
func (a App) AppID() string {
	if !a.IsClick() || a.Hook == "" {
		return ""
	}
	return a.Name + "_" + a.Hook + "_" + a.Version
}

// Icon returns the absolute path of the package icon, or "" when it is absent.
func (a App) Icon() string {
	if a.StableDir == "" {
		return ""
	}
	icon := filepath.Join(a.StableDir, "waymedia.png")
	if _, err := os.Stat(icon); err != nil {
		return ""
	}
	return icon
}

// readManifest finds the click manifest. An installed package keeps it in
// .click/info/<name>.manifest, not next to the payload; a plain manifest.json
// in the directory is honoured too, which is what an unpacked build tree has.
func readManifest(dir string) (name, version, hook string, ok bool) {
	paths := []string{filepath.Join(dir, "manifest.json")}
	if found, err := filepath.Glob(filepath.Join(dir, ".click", "info", "*.manifest")); err == nil {
		paths = append(paths, found...)
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var m struct {
			Name    string                       `json:"name"`
			Version string                       `json:"version"`
			Hooks   map[string]map[string]string `json:"hooks"`
		}
		if err := json.Unmarshal(raw, &m); err != nil || m.Name == "" || m.Version == "" {
			continue
		}
		for h, spec := range m.Hooks {
			if spec["desktop"] != "" {
				hook = h
				break
			}
		}
		return m.Name, m.Version, hook, true
	}
	return "", "", "", false
}

// stableDir prefers the sibling "current" symlink over a versioned directory,
// so a generated desktop file survives a package update.
//
// Both sides are resolved before comparing: a parent component of dir may
// itself be a symlink, and comparing a resolved path against an unresolved one
// would silently keep the versioned directory.
func stableDir(dir string) string {
	current := filepath.Join(filepath.Dir(dir), "current")
	target, err := filepath.EvalSymlinks(current)
	if err != nil {
		return dir
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil || target != resolved {
		return dir
	}
	return current
}
