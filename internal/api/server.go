// Package api exposes the REST surface (Spec 0005): list CRUD, /status,
// /classify and suggestions, all behind a mandatory Bearer token.
package api

import (
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Jondalar/mailscreener/internal/classify"
	"github.com/Jondalar/mailscreener/internal/lists"
)

// uiHTML is the self-contained browser UI (single page, vanilla JS). It is
// served unauthenticated at "/"; it asks the user for the API token and then
// calls the JSON API below with a Bearer header (token kept in localStorage).
//
//go:embed ui/index.html
var uiHTML []byte

// Health is the live daemon state the status endpoint reports. It is updated by
// the sweep loop and read by the API.
type Health struct {
	mu        sync.Mutex
	connected bool
	lastSweep time.Time
	lastErr   string
}

// SetConnected records connection state and an optional error.
func (h *Health) SetConnected(ok bool, err string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connected = ok
	h.lastErr = err
}

// SweptNow records a successful sweep.
func (h *Health) SweptNow() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastSweep = time.Now()
}

func (h *Health) snapshot() (bool, time.Time, string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.connected, h.lastSweep, h.lastErr
}

// Server is the HTTP API.
type Server struct {
	store   *lists.Store
	token   string
	version string
	started time.Time
	health  *Health
}

// New builds the API server.
func New(store *lists.Store, token, version string, health *Health) *Server {
	return &Server{store: store, token: token, version: version, started: time.Now(), health: health}
}

// Handler returns the HTTP handler: the browser UI is served unauthenticated at
// exactly "/", everything else goes through the Bearer-auth JSON API.
func (s *Server) Handler() http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("GET /status", s.handleStatus)
	api.HandleFunc("GET /lists/{kind}", s.handleListGet)
	api.HandleFunc("POST /lists/{kind}", s.handleListPost)
	api.HandleFunc("DELETE /lists/{kind}/{value}", s.handleListDelete)
	api.HandleFunc("POST /classify", s.handleClassify)
	api.HandleFunc("GET /suggestions", s.handleSuggestions)
	api.HandleFunc("POST /suggestions/apply", s.handleApply)

	root := http.NewServeMux()
	// "GET /{$}" matches only the exact root path, so it never shadows the
	// named API routes below. The page itself carries no secrets.
	root.HandleFunc("GET /{$}", s.handleUI)
	root.Handle("/", s.auth(api))
	return root
}

func (s *Server) handleUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(uiHTML)
}

// auth enforces the Bearer token on every request.
func (s *Server) auth(next http.Handler) http.Handler {
	want := "Bearer " + s.token
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("Authorization")
		if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	sizes := map[string]int{}
	for _, k := range lists.AllKinds {
		n, err := s.store.Count(k)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "count")
			return
		}
		sizes[string(k)] = n
	}
	connected, lastSweep, lastErr := s.health.snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"version":    s.version,
		"uptime":     time.Since(s.started).Round(time.Second).String(),
		"connected":  connected,
		"lastSweep":  zeroNil(lastSweep),
		"lastError":  lastErr,
		"listSizes":  sizes,
	})
}

func (s *Server) handleListGet(w http.ResponseWriter, r *http.Request) {
	k, ok := parseKind(w, r)
	if !ok {
		return
	}
	vals, err := s.store.All(k)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "read")
		return
	}
	if vals == nil {
		vals = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"kind": string(k), "entries": vals})
}

func (s *Server) handleListPost(w http.ResponseWriter, r *http.Request) {
	k, ok := parseKind(w, r)
	if !ok {
		return
	}
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Value == "" {
		writeErr(w, http.StatusBadRequest, "value required")
		return
	}
	if _, err := s.store.Add(k, body.Value, "api"); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"kind": string(k), "value": lists.Normalize(body.Value)})
}

func (s *Server) handleListDelete(w http.ResponseWriter, r *http.Request) {
	k, ok := parseKind(w, r)
	if !ok {
		return
	}
	removed, err := s.store.Remove(k, r.PathValue("value"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "delete")
		return
	}
	if !removed {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleClassify(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Sender          string `json:"sender"`
		ListID          string `json:"listId"`
		ListUnsubscribe string `json:"listUnsubscribe"`
		ListPost        string `json:"listPost"`
		ListHelp        string `json:"listHelp"`
		XGoogleGroupID  string `json:"xGoogleGroupId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body")
		return
	}
	snap, err := s.store.Snapshot()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "snapshot")
		return
	}
	v := classify.Classify(
		classify.Sender{Address: lists.Normalize(body.Sender)},
		classify.Headers{
			ListID: body.ListID, ListUnsubscribe: body.ListUnsubscribe,
			ListPost: body.ListPost, ListHelp: body.ListHelp, XGoogleGroupID: body.XGoogleGroupID,
		}, snap)
	writeJSON(w, http.StatusOK, map[string]any{"verdict": string(v)})
}

func (s *Server) handleSuggestions(w http.ResponseWriter, r *http.Request) {
	min := 5
	if v := r.URL.Query().Get("min"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			min = n
		}
	}
	sug, err := s.store.Suggest(min)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "suggest")
		return
	}
	if sug == nil {
		sug = []lists.Suggestion{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"suggestions": sug})
}

func (s *Server) handleApply(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind     string `json:"kind"`
		Wildcard string `json:"wildcard"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body")
		return
	}
	if err := s.store.ApplySuggestion(lists.Kind(body.Kind), body.Wildcard); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

func parseKind(w http.ResponseWriter, r *http.Request) (lists.Kind, bool) {
	k := lists.Kind(r.PathValue("kind"))
	for _, valid := range lists.AllKinds {
		if k == valid {
			return k, true
		}
	}
	writeErr(w, http.StatusNotFound, "unknown list kind")
	return "", false
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func zeroNil(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}
