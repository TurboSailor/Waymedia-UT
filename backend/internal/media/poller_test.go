package media

import (
	"testing"

	"waymedia/internal/waydroid"
)

// A state change is what triggers MPRIS PropertiesChanged, and the indicator
// (plus the shell) wakes up for each one. Position advances on every poll and
// must not count; anything a client renders must.
func TestSignificantChange(t *testing.T) {
	base := State{
		Reachable: true, HasSession: true, AppName: "Music",
		ContainerState: waydroid.SessionRunning,
		Session: Session{
			Package: "com.a", Status: StatusPlaying, Title: "T", Artist: "A",
			Actions: 1 << 5, PositionMS: 1000, HasState: true,
		},
	}
	mutate := func(f func(*State)) State {
		s := base
		f(&s)
		return s
	}

	tests := []struct {
		name string
		next State
		want bool
	}{
		{"identical", base, false},
		{"position only", mutate(func(s *State) { s.Session.PositionMS = 9000 }), false},
		{"status", mutate(func(s *State) { s.Session.Status = StatusPaused }), true},
		{"title", mutate(func(s *State) { s.Session.Title = "T2" }), true},
		{"artist", mutate(func(s *State) { s.Session.Artist = "A2" }), true},
		{"album", mutate(func(s *State) { s.Session.Album = "X" }), true},
		{"package", mutate(func(s *State) { s.Session.Package = "com.b" }), true},
		{"actions", mutate(func(s *State) { s.Session.Actions = 0 }), true},
		{"session gone", mutate(func(s *State) { s.HasSession = false }), true},
		{"unreachable", mutate(func(s *State) { s.Reachable = false }), true},
		{"container state", mutate(func(s *State) { s.ContainerState = waydroid.SessionStopped }), true},
		{"error appeared", mutate(func(s *State) { s.Err = "boom" }), true},
		{"app name resolved", mutate(func(s *State) { s.AppName = "Яндекс Музыка" }), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := significantChange(base, tc.next); got != tc.want {
				t.Errorf("significantChange = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPrettyPkg(t *testing.T) {
	tests := map[string]string{
		"ru.yandex.music":        "Music",
		"com.spotify.music":      "Music",
		"org.telegram.messenger": "Messenger",
		"single":                 "Single",
		"trailing.":              "Trailing.",
	}
	for in, want := range tests {
		if got := prettyPkg(in); got != want {
			t.Errorf("prettyPkg(%q) = %q, want %q", in, got, want)
		}
	}
}

// Real players (androidx.media3, e.g. Yandex Music with actions=524283)
// advertise ACTION_PLAY_PAUSE without ACTION_PLAY, and ignore a media key they
// never claimed — so Play has to become a toggle, but only when the toggle
// lands on "playing".
func TestEffectiveKey(t *testing.T) {
	const yandex = int64(524283) // PAUSE|PLAY_PAUSE|PREV|NEXT|STOP, no PLAY
	session := func(actions int64, status Status) State {
		return State{HasSession: true, Session: Session{Actions: actions, Status: status, HasState: true}}
	}

	tests := []struct {
		name string
		cmd  string
		st   State
		key  string
		send bool
	}{
		{"play without ACTION_PLAY while paused", CmdPlay, session(yandex, StatusPaused), CmdPlayPause, true},
		{"play without ACTION_PLAY while playing", CmdPlay, session(yandex, StatusPlaying), "", false},
		{"pause with ACTION_PAUSE", CmdPause, session(yandex, StatusPlaying), CmdPause, true},
		{"play with ACTION_PLAY", CmdPlay, session(actionPlay|actionPlayPause, StatusPaused), CmdPlay, true},
		{"pause without ACTION_PAUSE while playing", CmdPause, session(actionPlayPause, StatusPlaying), CmdPlayPause, true},
		{"pause without ACTION_PAUSE while paused", CmdPause, session(actionPlayPause, StatusPaused), "", false},
		// No ACTION_PLAY_PAUSE to fall back on: send what was asked and let
		// the session decide.
		{"play without any hint", CmdPlay, session(actionStop, StatusPaused), CmdPlay, true},
		{"next is never rewritten", CmdNext, session(yandex, StatusPlaying), CmdNext, true},
		{"play-pause passes through", CmdPlayPause, session(yandex, StatusPlaying), CmdPlayPause, true},
		{"no session at all", CmdPlay, State{}, CmdPlay, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, send := effectiveKey(tc.cmd, tc.st)
			if key != tc.key || send != tc.send {
				t.Errorf("effectiveKey = (%q, %v), want (%q, %v)", key, send, tc.key, tc.send)
			}
		})
	}
}

func TestParseNonLocalizedLabel(t *testing.T) {
	const dump = `Packages:
  Package [com.example] (abc):
    applicationInfo=ApplicationInfo{1 com.example}
    nonLocalizedLabel=Example Player icon=0x7f08
`
	if got := parseNonLocalizedLabel(dump); got != "Example Player" {
		t.Errorf("label = %q", got)
	}
	if got := parseNonLocalizedLabel("nonLocalizedLabel=null icon=0x1"); got != "" {
		t.Errorf("label = %q, want empty for null", got)
	}
	if got := parseNonLocalizedLabel("no label here"); got != "" {
		t.Errorf("label = %q, want empty", got)
	}
}
