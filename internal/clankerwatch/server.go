package clankerwatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultOutputLimit = 10 * 1024 * 1024

type API struct {
	store     *Store
	secretsMu sync.RWMutex
	secrets   map[string]map[string]string
	events    *eventHub
}

type Event struct {
	Type    string `json:"type"`
	RunID   int64  `json:"runId,omitempty"`
	Profile string `json:"profile,omitempty"`
}

func NewAPI(store *Store) *API {
	return &API{
		store:   store,
		secrets: map[string]map[string]string{},
		events:  newEventHub(),
	}
}

func (api *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", api.withHTTP(api.handleHealth))
	mux.HandleFunc("/api/events", api.handleEvents)
	mux.HandleFunc("/api/profiles", api.withHTTP(api.handleProfiles))
	mux.HandleFunc("/api/profiles/", api.withHTTP(api.handleProfileSubroute))
	mux.HandleFunc("/api/sessions", api.withHTTP(api.handleSessions))
	mux.HandleFunc("/api/sessions/", api.withHTTP(api.handleSessionSubroute))
	mux.HandleFunc("/api/query", api.withHTTP(api.handleQuery))
	mux.HandleFunc("/api/runs", api.withHTTP(api.handleRuns))
	mux.HandleFunc("/api/runs/", api.withHTTP(api.handleRunSubroute))
	return api.withCORS(mux)
}

func (api *API) handleSessions(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodGet:
		sessions, err := api.store.ListSessions(r.Context())
		if err != nil {
			return err
		}
		return writeJSON(w, http.StatusOK, sessions)
	case http.MethodPost:
		var input SessionInput
		if err := readJSON(r, &input); err != nil {
			return badRequest(err.Error())
		}
		session, err := api.store.CreateSession(r.Context(), input.Name)
		if err != nil {
			return err
		}
		api.events.emit(Event{Type: "sessions"})
		return writeJSON(w, http.StatusOK, session)
	default:
		return methodNotAllowed()
	}
}

func (api *API) handleSessionSubroute(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		return methodNotAllowed()
	}
	value := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/sessions/"), "/")
	session, err := api.store.FindSession(r.Context(), value)
	if err != nil {
		return notFound()
	}
	return writeJSON(w, http.StatusOK, session)
}

func (api *API) Serve(addr string) (*http.Server, net.Listener, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}
	server := &http.Server{Handler: api.Handler()}
	return server, listener, nil
}

func (api *API) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (api *API) withHTTP(handler func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			var httpErr httpError
			if errors.As(err, &httpErr) {
				writeError(w, httpErr.status, httpErr.message)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
		}
	}
}

func (api *API) handleHealth(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		return methodNotAllowed()
	}
	return writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (api *API) handleProfiles(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodGet:
		profiles, err := api.store.ListProfiles(r.Context(), api.unlockedMap())
		if err != nil {
			return err
		}
		return writeJSON(w, http.StatusOK, profiles)
	case http.MethodPost:
		var input ProfileInput
		if err := readJSON(r, &input); err != nil {
			return badRequest(err.Error())
		}
		profile, err := api.store.SaveProfile(r.Context(), input)
		if err != nil {
			return badRequest(err.Error())
		}
		profile.Unlocked = api.isUnlocked(profile.Name)
		api.events.emit(Event{Type: "profiles", Profile: profile.Name})
		return writeJSON(w, http.StatusOK, profile)
	default:
		return methodNotAllowed()
	}
}

func (api *API) handleProfileSubroute(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return methodNotAllowed()
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/profiles/"), "/")
	if len(parts) != 2 || parts[1] != "unlock" {
		return notFound()
	}
	name := parts[0]
	if _, err := api.store.GetProfile(r.Context(), name); err != nil {
		return notFound()
	}
	var input UnlockInput
	if err := readJSON(r, &input); err != nil {
		return badRequest(err.Error())
	}
	api.secretsMu.Lock()
	api.secrets[name] = copyMap(input.SecretEnv)
	api.secretsMu.Unlock()
	api.events.emit(Event{Type: "profiles", Profile: name})
	return writeJSON(w, http.StatusOK, map[string]any{"name": name, "unlocked": true})
}

func (api *API) handleQuery(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return methodNotAllowed()
	}
	var input QueryRequest
	if err := readJSON(r, &input); err != nil {
		return badRequest(err.Error())
	}
	response, status, err := api.RunQuery(r.Context(), input)
	if err != nil {
		return err
	}
	return writeJSON(w, status, response)
}

func (api *API) handleRuns(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		return methodNotAllowed()
	}
	runs, err := api.store.ListRuns(r.Context(), 100, r.URL.Query().Get("sessionId"))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, runs)
}

func (api *API) handleRunSubroute(w http.ResponseWriter, r *http.Request) error {
	rest := strings.TrimPrefix(r.URL.Path, "/api/runs/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || parts[0] == "" {
		return notFound()
	}
	runID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return badRequest("run id must be a number")
	}
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			return methodNotAllowed()
		}
		run, err := api.store.GetRun(r.Context(), runID)
		if err != nil {
			return notFound()
		}
		return writeJSON(w, http.StatusOK, run)
	}
	switch parts[1] {
	case "rows":
		if r.Method != http.MethodGet {
			return methodNotAllowed()
		}
		page := intQuery(r, "page", 1)
		pageSize := intQuery(r, "pageSize", 50)
		rows, err := api.store.GetRows(r.Context(), runID, page, pageSize)
		if err != nil {
			return err
		}
		return writeJSON(w, http.StatusOK, rows)
	case "annotations":
		if r.Method != http.MethodPost {
			return methodNotAllowed()
		}
		var input AnnotationInput
		if err := readJSON(r, &input); err != nil {
			return badRequest(err.Error())
		}
		annotation, err := api.store.AddAnnotation(r.Context(), input, runID)
		if err != nil {
			return badRequest(err.Error())
		}
		api.events.emit(Event{Type: "annotations", RunID: runID})
		return writeJSON(w, http.StatusOK, annotation)
	default:
		return notFound()
	}
}

func (api *API) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	api.events.serve(w, r)
}

func (api *API) RunQuery(ctx context.Context, input QueryRequest) (QueryResponse, int, error) {
	input.Profile = strings.TrimSpace(input.Profile)
	input.SessionID = strings.TrimSpace(input.SessionID)
	input.SQL = strings.TrimSpace(input.SQL)
	if input.Profile == "" {
		return QueryResponse{}, http.StatusBadRequest, badRequest("profile is required")
	}
	if input.SQL == "" {
		return QueryResponse{}, http.StatusBadRequest, badRequest("sql is required")
	}
	if strings.TrimSpace(input.Reason) == "" {
		input.Reason = "query from agent"
	}
	profile, err := api.store.GetProfile(ctx, input.Profile)
	if err != nil {
		return QueryResponse{}, http.StatusNotFound, notFound()
	}

	if !IsReadOnlySQL(input.SQL) {
		run, err := api.store.CreateRun(ctx, input.SessionID, profile.Name, input.SQL, input.Reason, "blocked")
		if err != nil {
			return QueryResponse{}, http.StatusInternalServerError, err
		}
		stderr := "clankerwatch blocked this query because it does not look read-only"
		run, err = api.store.FinishRun(ctx, run.ID, "blocked", 2, "", stderr, ParsedTable{})
		if err != nil {
			return QueryResponse{}, http.StatusInternalServerError, err
		}
		api.events.emit(Event{Type: "runs", RunID: run.ID, Profile: profile.Name})
		if input.SessionID != "" {
			_ = api.store.TouchSession(ctx, input.SessionID)
		}
		return QueryResponse{RunID: run.ID, SessionID: run.SessionID, Status: run.Status, ExitCode: run.ExitCode, Stderr: run.Stderr}, http.StatusForbidden, nil
	}

	run, err := api.store.CreateRun(ctx, input.SessionID, profile.Name, input.SQL, input.Reason, "running")
	if err != nil {
		return QueryResponse{}, http.StatusInternalServerError, err
	}
	api.events.emit(Event{Type: "runs", RunID: run.ID, Profile: profile.Name})

	exitCode, stdout, stderr := api.executeRun(ctx, profile, run.ID, input.SQL)
	status := "succeeded"
	if exitCode != 0 {
		status = "failed"
	}
	parsed := ParseTable(profile.Adapter, stdout, profile.MaxRows)
	run, err = api.store.FinishRun(ctx, run.ID, status, exitCode, stdout, stderr, parsed)
	if err != nil {
		return QueryResponse{}, http.StatusInternalServerError, err
	}
	api.events.emit(Event{Type: "runs", RunID: run.ID, Profile: profile.Name})
	if input.SessionID != "" {
		_ = api.store.TouchSession(ctx, input.SessionID)
	}
	return QueryResponse{
		RunID:     run.ID,
		SessionID: run.SessionID,
		Status:    run.Status,
		ExitCode:  run.ExitCode,
		Stdout:    run.Stdout,
		Stderr:    run.Stderr,
		Columns:   run.Columns,
		RowCount:  run.RowCount,
	}, http.StatusOK, nil
}

func (api *API) executeRun(ctx context.Context, profile Profile, runID int64, sql string) (int, string, string) {
	queryFile, err := writeQueryFile(runID, sql)
	if err != nil {
		return 1, "", err.Error()
	}
	defer os.Remove(queryFile)

	secrets := api.secretEnv(profile.Name)
	spec := BuildCommand(profile, queryFile, secrets)
	if spec.Name == "" {
		return 1, "", "profile command is required"
	}

	timeout := time.Duration(profile.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, spec.Name, spec.Args...)
	cmd.Env = append(os.Environ(), mapToEnv(spec.Env)...)
	if profile.Adapter == "generic" && !argsContainQueryFile(profile.Args) {
		cmd.Stdin = strings.NewReader(sql)
	}

	stdout := newLimitedBuffer(defaultOutputLimit)
	stderr := newLimitedBuffer(defaultOutputLimit)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Run()
	if execCtx.Err() == context.DeadlineExceeded {
		return 124, stdout.String(), stderr.String() + "\nclankerwatch timed out this query after " + strconv.Itoa(int(timeout/time.Millisecond)) + "ms"
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), stdout.String(), stderr.String()
		}
		return 1, stdout.String(), err.Error()
	}
	return 0, stdout.String(), stderr.String()
}

func writeQueryFile(runID int64, sql string) (string, error) {
	dir := filepath.Join(os.TempDir(), "clankerwatch")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("run-%d.sql", runID))
	return path, os.WriteFile(path, []byte(sql), 0o600)
}

func (api *API) unlockedMap() map[string]bool {
	api.secretsMu.RLock()
	defer api.secretsMu.RUnlock()
	out := map[string]bool{}
	for name := range api.secrets {
		out[name] = true
	}
	return out
}

func (api *API) isUnlocked(name string) bool {
	api.secretsMu.RLock()
	defer api.secretsMu.RUnlock()
	_, ok := api.secrets[name]
	return ok
}

func (api *API) secretEnv(name string) map[string]string {
	api.secretsMu.RLock()
	defer api.secretsMu.RUnlock()
	return copyMap(api.secrets[name])
}

type eventHub struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func newEventHub() *eventHub {
	return &eventHub{clients: map[chan []byte]struct{}{}}
}

func (h *eventHub) emit(event Event) {
	data, _ := json.Marshal(event)
	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.clients {
		select {
		case client <- data:
		default:
		}
	}
}

func (h *eventHub) serve(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not available")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan []byte, 16)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, ch)
		h.mu.Unlock()
		close(ch)
	}()

	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	for {
		select {
		case data := <-ch:
			fmt.Fprintf(w, "event: update\ndata: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(value)
}

func readJSON(r *http.Request, value any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	_ = writeJSON(w, status, map[string]string{"error": message})
}

type httpError struct {
	status  int
	message string
}

func (e httpError) Error() string {
	return e.message
}

func badRequest(message string) httpError {
	return httpError{status: http.StatusBadRequest, message: message}
}

func methodNotAllowed() httpError {
	return httpError{status: http.StatusMethodNotAllowed, message: "method not allowed"}
}

func notFound() httpError {
	return httpError{status: http.StatusNotFound, message: "not found"}
}

func intQuery(r *http.Request, key string, fallback int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func mapToEnv(values map[string]string) []string {
	env := make([]string, 0, len(values))
	for key, value := range values {
		env = append(env, key+"="+value)
	}
	return env
}

func copyMap(values map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range values {
		out[key] = value
	}
	return out
}

func isAllowedOrigin(origin string) bool {
	switch origin {
	case "http://127.0.0.1:5173",
		"http://cwatch.localhost",
		"https://cwatch.localhost":
		return true
	default:
		return false
	}
}
