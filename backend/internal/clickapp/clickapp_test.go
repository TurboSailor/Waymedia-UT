package clickapp

import (
	"os"
	"path/filepath"
	"testing"
)

// An installed click package keeps its manifest in .click/info, not next to
// the payload — reading only manifest.json is what made the daemon report
// version "dev" and generate a desktop file without an icon.
func TestReadManifestFromClickInfo(t *testing.T) {
	dir := t.TempDir()
	info := filepath.Join(dir, ".click", "info")
	if err := os.MkdirAll(info, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name":"waymedia.turbosailor","version":"0.1.0",
	  "hooks":{"waymedia":{"apparmor":"waymedia.apparmor","desktop":"waymedia.desktop"}}}`
	if err := os.WriteFile(filepath.Join(info, "waymedia.turbosailor.manifest"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	name, version, hook, ok := readManifest(dir)
	if !ok {
		t.Fatal("readManifest reported no manifest")
	}
	if name != "waymedia.turbosailor" || version != "0.1.0" || hook != "waymedia" {
		t.Fatalf("got (%q, %q, %q)", name, version, hook)
	}
}

func TestReadManifestPrefersPlainFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"),
		[]byte(`{"name":"p","version":"9","hooks":{"h":{"desktop":"h.desktop"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	name, version, hook, ok := readManifest(dir)
	if !ok || name != "p" || version != "9" || hook != "h" {
		t.Fatalf("got (%q, %q, %q, %v)", name, version, hook, ok)
	}
}

func TestReadManifestAbsent(t *testing.T) {
	if _, _, _, ok := readManifest(t.TempDir()); ok {
		t.Fatal("readManifest invented a manifest")
	}
}

// The desktop file hands paths to other processes, so they must survive the
// version directory being replaced by an update.
func TestStableDirPrefersCurrentSymlink(t *testing.T) {
	root := t.TempDir()
	ver := filepath.Join(root, "0.1.0")
	if err := os.MkdirAll(ver, 0o755); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(root, "current")
	if err := os.Symlink("0.1.0", current); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if got := stableDir(ver); got != current {
		t.Errorf("stableDir = %q, want %q", got, current)
	}
}

func TestStableDirWithoutSymlink(t *testing.T) {
	dir := t.TempDir()
	if got := stableDir(dir); got != dir {
		t.Errorf("stableDir = %q, want %q", got, dir)
	}
}

// stableDir must not point at "current" when that symlink belongs to a
// different version: after an update the old daemon keeps running from its own
// directory, and claiming the new one would advertise a foreign build.
func TestStableDirIgnoresForeignCurrent(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "0.1.0")
	newer := filepath.Join(root, "0.2.0")
	for _, d := range []string{old, newer} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("0.2.0", filepath.Join(root, "current")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if got := stableDir(old); got != old {
		t.Errorf("stableDir = %q, want %q", got, old)
	}
}

func TestAppAccessorsOnZeroValue(t *testing.T) {
	var a App
	if a.IsClick() {
		t.Error("zero App claims to be a click package")
	}
	if a.AppID() != "" || a.Icon() != "" {
		t.Errorf("AppID = %q, Icon = %q", a.AppID(), a.Icon())
	}
}

func TestAppIconMissingFile(t *testing.T) {
	a := App{Dir: t.TempDir(), StableDir: t.TempDir(), Name: "p", Version: "1", Hook: "h"}
	if got := a.Icon(); got != "" {
		t.Errorf("Icon = %q, want empty", got)
	}
}
