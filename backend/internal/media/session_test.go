package media

import (
	"os"
	"testing"
)

func TestParseDumpRealDevice(t *testing.T) {
	raw, err := os.ReadFile("../waydroid/testdata/dumpsys_media_session.txt")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	d := ParseDump(string(raw))

	// The global priority session (telecom) sits above the stack in the same
	// shape; picking it up would show a phantom player whenever nothing plays.
	if len(d.Sessions) != 1 {
		t.Fatalf("sessions = %d, want 1 (only the stack): %+v", len(d.Sessions), d.Sessions)
	}
	if d.ButtonPackage != "ru.yandex.music" {
		t.Errorf("button package = %q", d.ButtonPackage)
	}

	s, ok := d.Current()
	if !ok {
		t.Fatal("Current() found no session")
	}
	if s.Package != "ru.yandex.music" {
		t.Errorf("package = %q", s.Package)
	}
	if !s.Active {
		t.Error("active = false")
	}
	if s.Status != StatusPlaying {
		t.Errorf("status = %q, want Playing", s.Status)
	}
	if s.Title != "Always on My Mind" {
		t.Errorf("title = %q", s.Title)
	}
	if s.Artist != "Michael Bublé" {
		t.Errorf("artist = %q", s.Artist)
	}
	if s.Album != "" {
		t.Errorf("album = %q, want empty (dump printed null)", s.Album)
	}
	// "buffered position=0" must not be mistaken for "position=".
	if s.PositionMS != 223886 {
		t.Errorf("position = %d, want 223886", s.PositionMS)
	}
	if !s.CanPlay() || !s.CanPause() || !s.CanNext() || !s.CanPrevious() || !s.CanStop() {
		t.Errorf("actions %d decoded as play=%v pause=%v next=%v prev=%v stop=%v",
			s.Actions, s.CanPlay(), s.CanPause(), s.CanNext(), s.CanPrevious(), s.CanStop())
	}
}

func TestCurrentSkipsStatelessSessions(t *testing.T) {
	d := Dump{Sessions: []Session{
		{Package: "com.android.server.telecom"},
		{Package: "com.player", HasState: true, Active: true, Status: StatusPaused},
	}}
	s, ok := d.Current()
	if !ok || s.Package != "com.player" {
		t.Fatalf("Current() = %+v, %v", s, ok)
	}
}

func TestCurrentPrefersButtonSession(t *testing.T) {
	d := Dump{
		ButtonPackage: "com.second",
		Sessions: []Session{
			{Package: "com.first", HasState: true, Active: true, Status: StatusPlaying},
			{Package: "com.second", HasState: true, Status: StatusPaused},
		},
	}
	s, _ := d.Current()
	if s.Package != "com.second" {
		t.Fatalf("package = %q, want the media button session", s.Package)
	}
}

func TestCurrentEmpty(t *testing.T) {
	s, ok := Dump{}.Current()
	if ok {
		t.Fatalf("Current() reported a session: %+v", s)
	}
	if s.Status != StatusStopped {
		t.Errorf("status = %q, want Stopped", s.Status)
	}
}

func TestParseMetadata(t *testing.T) {
	tests := []struct {
		name                 string
		in                   string
		title, artist, album string
	}{
		{"null", " null", "", "", ""},
		{"three fields", " size=18, description=Track, Artist, Album", "Track", "Artist", "Album"},
		{"null tail", " size=3, description=Track, Artist, null", "Track", "Artist", ""},
		{"title only", " size=1, description=Track", "Track", "", ""},
		{"title and artist", " size=2, description=Track, Artist", "Track", "Artist", ""},
		// A comma inside the track name is indistinguishable from a field
		// separator; keeping the last two fields as artist/album is the split
		// that damages the least.
		{"comma in title", " size=4, description=Hello, Goodbye, The Beatles, Revolver",
			"Hello, Goodbye", "The Beatles", "Revolver"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			title, artist, album := parseMetadata(tc.in)
			if title != tc.title || artist != tc.artist || album != tc.album {
				t.Errorf("got (%q, %q, %q), want (%q, %q, %q)",
					title, artist, album, tc.title, tc.artist, tc.album)
			}
		})
	}
}

func TestStatusOf(t *testing.T) {
	tests := map[int]Status{
		androidStateNone:           StatusStopped,
		androidStateStopped:        StatusStopped,
		androidStatePaused:         StatusPaused,
		androidStatePlaying:        StatusPlaying,
		androidStateBuffering:      StatusPlaying,
		androidStateSkipToNext:     StatusPlaying,
		androidStateSkipToPrevious: StatusPlaying,
		androidStateError:          StatusStopped,
		99:                         StatusStopped,
	}
	for state, want := range tests {
		if got := statusOf(state); got != want {
			t.Errorf("statusOf(%d) = %q, want %q", state, got, want)
		}
	}
}

func TestParseDumpNoPlayer(t *testing.T) {
	const out = `MEDIA SESSION SERVICE (dumpsys media_session)

Global priority session is com.android.server.telecom/HeadsetMediaButton (userId=0)
  HeadsetMediaButton com.android.server.telecom/HeadsetMediaButton (userId=0)
    package=com.android.server.telecom
    active=false
    state=null
    metadata: null
User Records:
Record for full_user=0
  Media button session is null
  Sessions Stack - have 0 sessions:
Audio playback (lastly played comes first)
`
	d := ParseDump(out)
	if len(d.Sessions) != 0 {
		t.Fatalf("sessions = %+v, want none", d.Sessions)
	}
	if _, ok := d.Current(); ok {
		t.Error("Current() found a session in an idle dump")
	}
}

func TestParseDumpMultipleSessions(t *testing.T) {
	const out = `User Records:
  Media button session is com.b/tag-b (userId=0)
  Sessions Stack - have 2 sessions:
    tag-a com.a/tag-a (userId=0)
      package=com.a
      active=true
      state=PlaybackState {state=2, position=1000, buffered position=0, speed=0.0, actions=822, error=null}
      metadata: size=5, description=A, AA, null
    tag-b com.b/tag-b (userId=0)
      package=com.b
      active=true
      state=PlaybackState {state=3, position=2000, buffered position=0, speed=1.0, actions=513, error=null}
      metadata: size=5, description=B, BB, null
Audio playback (lastly played comes first)
`
	d := ParseDump(out)
	if len(d.Sessions) != 2 {
		t.Fatalf("sessions = %d, want 2", len(d.Sessions))
	}
	s, ok := d.Current()
	if !ok || s.Package != "com.b" || s.Status != StatusPlaying || s.PositionMS != 2000 {
		t.Fatalf("Current() = %+v, %v", s, ok)
	}
	// actions=513 is ACTION_STOP|ACTION_PLAY_PAUSE: play/pause both allowed,
	// skipping is not.
	if !s.CanPlay() || !s.CanPause() || s.CanNext() || s.CanPrevious() || !s.CanStop() {
		t.Errorf("actions %d decoded wrong: %+v", s.Actions, s)
	}
}
