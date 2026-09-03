// Package media turns Android's media session state into something the host
// session can publish over MPRIS, and turns MPRIS calls back into Android
// media key dispatches.
//
// `dumpsys media_session` is the only view on the container's playback that a
// shell offers, so the state below is exactly what that text carries — no
// album art, no track length, and a title that has been flattened into one
// comma separated line (see parseMetadata).
package media

import (
	"strconv"
	"strings"
)

// Status is the playback status in MPRIS spelling; the daemon publishes it
// verbatim and the HTTP API repeats it.
type Status string

const (
	StatusPlaying Status = "Playing"
	StatusPaused  Status = "Paused"
	StatusStopped Status = "Stopped"
)

// Android PlaybackState.STATE_* values, as printed by dumpsys.
const (
	androidStateNone            = 0
	androidStateStopped         = 1
	androidStatePaused          = 2
	androidStatePlaying         = 3
	androidStateFastForwarding  = 4
	androidStateRewinding       = 5
	androidStateBuffering       = 6
	androidStateError           = 7
	androidStateConnecting      = 8
	androidStateSkipToPrevious  = 9
	androidStateSkipToNext      = 10
	androidStateSkipToQueueItem = 11
)

// PlaybackStateCompat.ACTION_* bits of the actions bitmask.
const (
	actionStop           int64 = 1 << 0
	actionPause          int64 = 1 << 1
	actionPlay           int64 = 1 << 2
	actionSkipToPrevious int64 = 1 << 4
	actionSkipToNext     int64 = 1 << 5
	actionPlayPause      int64 = 1 << 9
)

// Session is one Android media session as dumpsys describes it.
type Session struct {
	Tag        string
	Package    string
	Active     bool
	Status     Status
	Title      string
	Artist     string
	Album      string
	PositionMS int64
	Actions    int64
	// HasState is false when dumpsys printed "state=null", i.e. the session
	// exists but has never reported playback. Such a session is a placeholder
	// (the telecom one always is) and must not win over a real player.
	HasState bool
}

func (s Session) CanPlay() bool  { return s.Actions&(actionPlay|actionPlayPause) != 0 }
func (s Session) CanPause() bool { return s.Actions&(actionPause|actionPlayPause) != 0 }

// CanNext is strictly ACTION_SKIP_TO_NEXT: the command path is a
// KEYCODE_MEDIA_NEXT media key, which a session advertising only
// ACTION_SKIP_TO_QUEUE_ITEM (jumping within its queue) never handles.
func (s Session) CanNext() bool     { return s.Actions&actionSkipToNext != 0 }
func (s Session) CanPrevious() bool { return s.Actions&actionSkipToPrevious != 0 }
func (s Session) CanStop() bool     { return s.Actions&actionStop != 0 }

// Dump is the parsed form of one `dumpsys media_session` run.
type Dump struct {
	// Sessions is the session stack in Android's own priority order.
	Sessions []Session
	// ButtonPackage is the package of the session Android would hand a media
	// key to ("Media button session is <pkg>/<tag>"). It is the closest thing
	// to "the player the user means" that the dump offers.
	ButtonPackage string
	ButtonTag     string
}

// Current picks the session to publish: the media button session when it is in
// the stack, otherwise the first active one, otherwise the first one that has
// ever reported playback.
//
// Sessions without state are skipped entirely — com.android.server.telecom
// always keeps one, and publishing it would show an empty player whenever
// nothing else plays.
func (d Dump) Current() (Session, bool) {
	var firstActive, firstWithState *Session
	for i := range d.Sessions {
		s := &d.Sessions[i]
		if !s.HasState {
			continue
		}
		if d.ButtonPackage != "" && s.Package == d.ButtonPackage {
			if d.ButtonTag == "" || d.ButtonTag == s.Tag {
				return *s, true
			}
		}
		if s.Active && firstActive == nil {
			firstActive = s
		}
		if firstWithState == nil {
			firstWithState = s
		}
	}
	switch {
	case firstActive != nil:
		return *firstActive, true
	case firstWithState != nil:
		return *firstWithState, true
	}
	return Session{Status: StatusStopped}, false
}

// ParseDump reads the output of `dumpsys media_session`.
//
// Only the "Sessions Stack" block is considered. The dump also prints a global
// priority session above it (telecom's media button handler) in the same shape,
// which is not a player and would otherwise be mistaken for one.
func ParseDump(out string) Dump {
	var (
		d       Dump
		inStack bool
		cur     *Session
	)
	flush := func() {
		if cur != nil {
			d.Sessions = append(d.Sessions, *cur)
			cur = nil
		}
	}

	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " "))

		if pkg, tag, ok := parseButtonSession(trimmed); ok {
			d.ButtonPackage, d.ButtonTag = pkg, tag
			continue
		}
		if strings.HasPrefix(trimmed, "Sessions Stack") {
			flush()
			inStack = true
			continue
		}
		if !inStack {
			continue
		}
		// The stack ends at the next unindented heading ("Audio playback",
		// "Media session config", or the next user record).
		if trimmed != "" && indent < 4 {
			flush()
			inStack = false
			continue
		}
		if trimmed == "" {
			continue
		}

		// A session header sits at indent 4, its fields at indent 6.
		if indent == 4 {
			flush()
			s := Session{Status: StatusStopped}
			s.Tag, s.Package = parseSessionHeader(trimmed)
			cur = &s
			continue
		}
		if cur == nil {
			continue
		}
		applyField(cur, trimmed)
	}
	flush()
	return d
}

// parseButtonSession reads "Media button session is <pkg>/<tag> (userId=0)".
func parseButtonSession(line string) (pkg, tag string, ok bool) {
	rest, ok := strings.CutPrefix(line, "Media button session is ")
	if !ok {
		return "", "", false
	}
	rest = strings.TrimSpace(rest)
	if rest == "" || rest == "null" {
		return "", "", false
	}
	if i := strings.Index(rest, " (userId="); i >= 0 {
		rest = rest[:i]
	}
	pkg, tag, found := strings.Cut(rest, "/")
	if !found {
		return "", "", false
	}
	return pkg, tag, true
}

// parseSessionHeader reads "<tag> <pkg>/<tag> (userId=0)". The tag is repeated
// after the package, so the package is taken from the "pkg/tag" pair and the
// leading field is only a fallback for the tag.
func parseSessionHeader(line string) (tag, pkg string) {
	if i := strings.Index(line, " (userId="); i >= 0 {
		line = line[:i]
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", ""
	}
	last := fields[len(fields)-1]
	if p, t, ok := strings.Cut(last, "/"); ok {
		pkg = p
		tag = t
	}
	if tag == "" && len(fields) > 1 {
		tag = fields[0]
	}
	return tag, pkg
}

func applyField(s *Session, line string) {
	switch {
	case strings.HasPrefix(line, "package="):
		s.Package = strings.TrimSpace(strings.TrimPrefix(line, "package="))
	case strings.HasPrefix(line, "active="):
		s.Active = strings.TrimSpace(strings.TrimPrefix(line, "active=")) == "true"
	case strings.HasPrefix(line, "state="):
		applyPlaybackState(s, strings.TrimPrefix(line, "state="))
	case strings.HasPrefix(line, "metadata:"):
		s.Title, s.Artist, s.Album = parseMetadata(strings.TrimPrefix(line, "metadata:"))
	}
}

// applyPlaybackState reads "PlaybackState {state=3, position=223886, ...}".
// Values are pulled by key because the field list differs between Android
// versions and custom actions inject commas and brackets of their own.
func applyPlaybackState(s *Session, val string) {
	val = strings.TrimSpace(val)
	if val == "" || val == "null" {
		return
	}
	s.HasState = true
	if n, ok := intField(val, "state="); ok {
		s.Status = statusOf(int(n))
	}
	if n, ok := intField(val, "position="); ok {
		s.PositionMS = n
	}
	if n, ok := intField(val, "actions="); ok {
		s.Actions = n
	}
}

// intField reads the integer that follows key. "buffered position=" also ends
// in "position=", so the search anchors on the preceding delimiter.
func intField(s, key string) (int64, bool) {
	for i := 0; i+len(key) <= len(s); {
		idx := strings.Index(s[i:], key)
		if idx < 0 {
			return 0, false
		}
		at := i + idx
		if at > 0 {
			if prev := s[at-1]; prev != ' ' && prev != '{' && prev != ',' {
				i = at + len(key)
				continue
			}
		}
		rest := s[at+len(key):]
		end := 0
		for end < len(rest) && (rest[end] == '-' || (rest[end] >= '0' && rest[end] <= '9')) {
			end++
		}
		if end == 0 {
			i = at + len(key)
			continue
		}
		n, err := strconv.ParseInt(rest[:end], 10, 64)
		if err != nil {
			i = at + len(key)
			continue
		}
		return n, true
	}
	return 0, false
}

// parseMetadata reads "size=18, description=Title, Artist, Album".
//
// MediaMetadata.toString() flattens the description into one comma separated
// line, so a title that itself contains ", " cannot be told apart from the
// artist boundary. Android always prints exactly three description fields, so
// the last two are taken as artist and album and everything before them is the
// title — the only split that keeps a comma inside a track name harmless.
func parseMetadata(val string) (title, artist, album string) {
	val = strings.TrimSpace(val)
	if val == "" || val == "null" {
		return "", "", ""
	}
	idx := strings.Index(val, "description=")
	if idx < 0 {
		return "", "", ""
	}
	desc := strings.TrimSpace(val[idx+len("description="):])
	if desc == "" || desc == "null" {
		return "", "", ""
	}
	parts := strings.Split(desc, ", ")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
		if parts[i] == "null" {
			parts[i] = ""
		}
	}
	switch len(parts) {
	case 1:
		return parts[0], "", ""
	case 2:
		return parts[0], parts[1], ""
	default:
		return strings.Join(parts[:len(parts)-2], ", "), parts[len(parts)-2], parts[len(parts)-1]
	}
}

func statusOf(state int) Status {
	switch state {
	case androidStatePlaying, androidStateFastForwarding, androidStateRewinding,
		androidStateBuffering, androidStateConnecting,
		androidStateSkipToPrevious, androidStateSkipToNext, androidStateSkipToQueueItem:
		return StatusPlaying
	case androidStatePaused:
		return StatusPaused
	case androidStateNone, androidStateStopped, androidStateError:
		return StatusStopped
	default:
		return StatusStopped
	}
}
