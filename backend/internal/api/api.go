// Package api is the daemon's local HTTP interface.
//
// The QML front end is a confined click application: it cannot join the
// daemon's process, and talking D-Bus from QML would duplicate the whole MPRIS
// model. A loopback JSON API is the same pattern the rest of the phone's
// unprivileged helpers use, and it keeps the UI a thin renderer of daemon
// state.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"waymedia/internal/config"
	"waymedia/internal/indicator"
	"waymedia/internal/media"
	"waymedia/internal/mpris"
	"waymedia/internal/waydroid"
)

// Addr is loopback-only: nothing outside the phone has any business driving
// playback, and binding a wildcard address would need a networking policy the
// application does not otherwise use.
const Addr = "127.0.0.1:21980"

// Bridge is the daemon state the API reports and mutates.
type Bridge interface {
	State() media.State
	Dispatch(ctx context.Context, cmd string) error
	// Exported reports whether the MPRIS bus name is currently owned.
	Exported() bool
	Registration() indicator.Registration
	// Transport names the channel into the container, for diagnostics.
	Transport() string
	// Apply switches the bridge on or off; it returns once the new settings
	// are in effect, so the response reports the resulting state.
	Apply(s config.Settings)
}

type Server struct {
	log      *slog.Logger
	bridge   Bridge
	settings *config.Store
	version  string
	started  time.Time
	srv      *http.Server
}

func New(log *slog.Logger, bridge Bridge, settings *config.Store, version string) *Server {
	s := &Server{
		log:      log,
		bridge:   bridge,
		settings: settings,
		version:  version,
		started:  time.Now(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/cmd", s.handleCmd)
	mux.HandleFunc("/api/settings", s.handleSettings)
	s.srv = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		// Long enough to cover a dispatch that waits on a busy container.
		WriteTimeout: 15 * time.Second,
	}
	return s
}

// Serve listens and blocks until ctx is cancelled.
func (s *Server) Serve(ctx context.Context) error {
	ln, err := net.Listen("tcp", Addr)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.srv.Shutdown(shutdown)
	}()
	s.log.Info("api listening", "addr", Addr)
	if err := s.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// ------------------------------------------------------------- wire types ---

type statusResponse struct {
	Version   string         `json:"version"`
	UptimeSec int64          `json:"uptime_sec"`
	Waydroid  waydroidStatus `json:"waydroid"`
	Bridge    bridgeStatus   `json:"bridge"`
	Now       nowPlaying     `json:"now"`
	Settings  settingsBody   `json:"settings"`
}

type waydroidStatus struct {
	Session    string `json:"session"`
	Reachable  bool   `json:"reachable"`
	Transport  string `json:"transport"`
	Error      string `json:"error"`
	LastPollMS int64  `json:"last_poll_ms"`
}

type bridgeStatus struct {
	Enabled           bool   `json:"enabled"`
	MprisExported     bool   `json:"mpris_exported"`
	BusName           string `json:"bus_name"`
	DesktopID         string `json:"desktop_id"`
	DesktopRegistered bool   `json:"desktop_registered"`
	Error             string `json:"error"`
}

type nowPlaying struct {
	HasSession  bool   `json:"has_session"`
	Package     string `json:"package"`
	AppName     string `json:"app_name"`
	Status      string `json:"status"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	PositionMS  int64  `json:"position_ms"`
	CanPlay     bool   `json:"can_play"`
	CanPause    bool   `json:"can_pause"`
	CanNext     bool   `json:"can_next"`
	CanPrevious bool   `json:"can_previous"`
	CanStop     bool   `json:"can_stop"`
}

type settingsBody struct {
	Enabled    bool `json:"enabled"`
	PollMS     int  `json:"poll_ms"`
	IdlePollMS int  `json:"idle_poll_ms"`
}

// settingsPatch mirrors settingsBody with pointers so a partial write leaves
// untouched fields alone.
type settingsPatch struct {
	Enabled    *bool `json:"enabled"`
	PollMS     *int  `json:"poll_ms"`
	IdlePollMS *int  `json:"idle_poll_ms"`
}

type cmdRequest struct {
	Action string `json:"action"`
}

type cmdResponse struct {
	OK    bool       `json:"ok"`
	Error string     `json:"error,omitempty"`
	Now   nowPlaying `json:"now"`
}

// --------------------------------------------------------------- handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.status())
}

func (s *Server) handleCmd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, cmdResponse{Error: "POST only"})
		return
	}
	var req cmdRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, cmdResponse{Error: "malformed body"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	switch err := s.bridge.Dispatch(ctx, req.Action); {
	case err == nil:
		writeJSON(w, http.StatusOK, cmdResponse{OK: true, Now: now(s.bridge.State())})
	case errors.Is(err, media.ErrUnknownCommand):
		writeJSON(w, http.StatusBadRequest, cmdResponse{Error: err.Error()})
	default:
		// Anything else means the container did not take the command: adbd
		// gone, session stopped, or the shell timed out.
		s.log.Warn("dispatch failed", "action", req.Action, "err", err)
		writeJSON(w, http.StatusServiceUnavailable, cmdResponse{Error: err.Error(), Now: now(s.bridge.State())})
	}
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}
	var patch settingsPatch
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "malformed body"})
		return
	}
	next := s.settings.Get()
	if patch.Enabled != nil {
		next.Enabled = *patch.Enabled
	}
	if patch.PollMS != nil {
		next.PollMS = *patch.PollMS
	}
	if patch.IdlePollMS != nil {
		next.IdlePollMS = *patch.IdlePollMS
	}
	if err := s.settings.Set(next); err != nil {
		s.log.Error("settings save failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.bridge.Apply(s.settings.Get())
	writeJSON(w, http.StatusOK, s.status())
}

func (s *Server) status() statusResponse {
	st := s.bridge.State()
	set := s.settings.Get()
	reg := s.bridge.Registration()

	var lastPoll int64
	if !st.LastPoll.IsZero() {
		lastPoll = st.LastPoll.UnixMilli()
	}
	session := st.ContainerState
	if session == "" {
		session = waydroid.SessionUnknown
	}
	return statusResponse{
		Version:   s.version,
		UptimeSec: int64(time.Since(s.started).Seconds()),
		Waydroid: waydroidStatus{
			Session:    string(session),
			Reachable:  st.Reachable,
			Transport:  s.bridge.Transport(),
			Error:      st.Err,
			LastPollMS: lastPoll,
		},
		Bridge: bridgeStatus{
			Enabled:           set.Enabled,
			MprisExported:     s.bridge.Exported(),
			BusName:           mpris.BusName,
			DesktopID:         indicator.FileName,
			DesktopRegistered: reg.Registered,
			Error:             reg.Err,
		},
		Now:      now(st),
		Settings: settingsBody{Enabled: set.Enabled, PollMS: set.PollMS, IdlePollMS: set.IdlePollMS},
	}
}

func now(st media.State) nowPlaying {
	if !st.HasSession {
		return nowPlaying{Status: string(media.StatusStopped)}
	}
	s := st.Session
	return nowPlaying{
		HasSession:  true,
		Package:     s.Package,
		AppName:     st.AppName,
		Status:      string(s.Status),
		Title:       s.Title,
		Artist:      s.Artist,
		Album:       s.Album,
		PositionMS:  s.PositionMS,
		CanPlay:     s.CanPlay(),
		CanPause:    s.CanPause(),
		CanNext:     s.CanNext(),
		CanPrevious: s.CanPrevious(),
		CanStop:     s.CanStop(),
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
