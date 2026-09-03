package indicator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"waymedia/internal/clickapp"
)

// The indicator resolves the player through this file, so a malformed entry
// costs the lock screen controls entirely — GLib rejects entries without
// [Desktop Entry]/Type, and an unresolvable Exec makes DesktopAppInfo fail.
func TestContentIsResolvableDesktopEntry(t *testing.T) {
	got := content(clickapp.App{})
	for _, want := range []string{"[Desktop Entry]\n", "Type=Application\n", "Name=" + PlayerName + "\n", "Exec=/bin/true\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("content() missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Hidden=true") {
		t.Error("Hidden=true would make GLib skip the entry")
	}
	if strings.Contains(got, "Icon=") {
		t.Errorf("content() advertises an icon without a package:\n%s", got)
	}
}

func TestContentUsesClickAppID(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "waymedia.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := clickapp.App{
		Dir:       dir,
		StableDir: dir,
		Name:      "waymedia.turbosailor",
		Version:   "0.2.3",
		Hook:      "waymedia",
	}

	got := content(app)
	// lomiri-app-launch takes <package>_<hook>_<version>.
	if want := "Exec=lomiri-app-launch waymedia.turbosailor_waymedia_0.2.3\n"; !strings.Contains(got, want) {
		t.Errorf("content() missing %q:\n%s", want, got)
	}
	if want := "Icon=" + filepath.Join(dir, "waymedia.png") + "\n"; !strings.Contains(got, want) {
		t.Errorf("content() missing %q:\n%s", want, got)
	}
}

func TestEnsureWritesAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	reg := Ensure(clickapp.App{})
	if !reg.Registered || reg.Err != "" {
		t.Fatalf("Ensure() = %+v", reg)
	}
	want := filepath.Join(dir, "applications", FileName)
	if reg.Path != want {
		t.Fatalf("path = %q, want %q", reg.Path, want)
	}
	raw, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	info, err := os.Stat(want)
	if err != nil {
		t.Fatal(err)
	}
	first := info.ModTime()

	// A rewrite on every start would invalidate GLib's cache for nothing.
	if reg2 := Ensure(clickapp.App{}); !reg2.Registered {
		t.Fatalf("second Ensure() = %+v", reg2)
	}
	info2, err := os.Stat(want)
	if err != nil {
		t.Fatal(err)
	}
	if !info2.ModTime().Equal(first) {
		t.Error("Ensure() rewrote an up-to-date file")
	}
	if string(raw) != content(clickapp.App{}) {
		t.Error("written content differs from content()")
	}
	// No leftover temporary files: the indicator scans the directory.
	entries, err := os.ReadDir(filepath.Dir(want))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("applications dir holds %d entries, want 1", len(entries))
	}
}

func TestEnsureRefreshesStaleFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	path := filepath.Join(dir, "applications", FileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[Desktop Entry]\nName=old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if reg := Ensure(clickapp.App{}); !reg.Registered {
		t.Fatalf("Ensure() = %+v", reg)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != content(clickapp.App{}) {
		t.Errorf("stale file kept:\n%s", raw)
	}
}
