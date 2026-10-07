package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"runic/internal/config"
	"runic/internal/db"
	"runic/internal/executor"
)

//go:embed all:web/dist
var uiContent embed.FS

type Server struct {
	cfg       *config.Config
	db        *db.DB
	runner    *executor.Runner
	sched     *executor.Scheduler
	reloadMu  sync.Mutex
	startTime time.Time
}

func Serve(cfg *config.Config, runner *executor.Runner, d *db.DB, sched *executor.Scheduler) {
	s := &Server{
		cfg:       cfg,
		db:        d,
		runner:    runner,
		sched:     sched,
		startTime: time.Now(),
	}

	os.MkdirAll(cfg.LogDir, 0755)

	fmt.Printf("[server] listening on %s:%s\n", cfg.Host, cfg.Port)
	srv := &http.Server{
		Addr:         cfg.Host + ":" + cfg.Port,
		Handler:      s.handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "[server] fatal: %v\n", err)
		os.Exit(1)
	}
}

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("GET /api/logs/{hid}", s.handleLogs)
	mux.HandleFunc("GET /api/actions", s.handleListActions)
	mux.HandleFunc("GET /api/actions/{id}", s.handleGetAction)
	mux.HandleFunc("POST /api/actions/{id}/trigger", s.handleTriggerAction)
	mux.HandleFunc("POST /api/clean", s.handleClean)
	mux.HandleFunc("POST /api/reload", s.handleReload)
	mux.HandleFunc("GET /api/system", s.handleSystem)

	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	// Try to serve the file from the embedded dist directory
	f, err := uiContent.ReadFile("web/dist/" + path)
	if err != nil {
		// SPA fallback: serve index.html for any non-file route
		f, err = uiContent.ReadFile("web/dist/index.html")
		if err != nil {
			http.Error(w, "UI not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(f)
		return
	}
	// Determine content type from extension
	ct := "application/octet-stream"
	switch {
	case strings.HasSuffix(path, ".html"):
		ct = "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		ct = "application/javascript"
	case strings.HasSuffix(path, ".css"):
		ct = "text/css"
	case strings.HasSuffix(path, ".json"):
		ct = "application/json"
	case strings.HasSuffix(path, ".svg"):
		ct = "image/svg+xml"
	case strings.HasSuffix(path, ".png"):
		ct = "image/png"
	case strings.HasSuffix(path, ".ico"):
		ct = "image/x-icon"
	case strings.HasSuffix(path, ".woff"):
		ct = "font/woff"
	case strings.HasSuffix(path, ".woff2"):
		ct = "font/woff2"
	}
	w.Header().Set("Content-Type", ct)
	w.Write(f)
}

const defaultHistoryLimit = 500

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("history_ids")
	actionID := r.URL.Query().Get("action_id")
	systemParam := r.URL.Query().Get("system")
	var entries []db.HistoryEntry
	var err error

	if idStr != "" {
		parts := strings.Split(idStr, ",")
		var ids []int64
		for _, p := range parts {
			if id, err := strconv.ParseInt(p, 10, 64); err == nil {
				ids = append(ids, id)
			}
		}
		entries, err = s.db.GetHistoryByIDs(ids)
	} else {
		entries, err = s.db.ListHistory(defaultHistoryLimit)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []db.HistoryEntry{}
	}

	if actionID != "" {
		filtered := make([]db.HistoryEntry, 0)
		for _, e := range entries {
			if e.ActionID == actionID {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	} else if systemParam != "" {
		isSystem := systemParam == "true"
		filtered := make([]db.HistoryEntry, 0)
		for _, e := range entries {
			sys := strings.HasPrefix(e.ActionID, "@system/")
			if isSystem == sys {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("hid"), 10, 64)
	if err != nil {
		http.Error(w, "invalid log id", http.StatusBadRequest)
		return
	}
	entry, err := s.db.GetHistoryByID(id)
	if err != nil {
		http.Error(w, "log not found", http.StatusNotFound)
		return
	}
	data, err := os.ReadFile(entry.LogFilePath)
	if err != nil {
		http.Error(w, "log file not readable", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data)
}

func (s *Server) handleGetAction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing action id", http.StatusBadRequest)
		return
	}
	cfg := s.runner.Config()
	def, err := executor.LoadAction(cfg.Actions, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	executor.NormalizeAction(def, cfg.Timeout)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(def)
}

func (s *Server) handleTriggerAction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing action id", http.StatusBadRequest)
		return
	}

	payload := ""
	if r.ContentLength > 0 {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err == nil {
			payload = string(body)
		}
	}

	historyID, err := s.runner.RunAction(context.Background(), s.db, s.runner.Config().LogDir, id, payload)
	if err != nil {
		if errors.Is(err, executor.ErrPaused) {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		if errors.Is(err, executor.ErrConcurrencyLimitReached) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "queued",
		"action_id":  id,
		"history_id": historyID,
	})
}

func (s *Server) handleListActions(w http.ResponseWriter, r *http.Request) {
	isSystem := r.URL.Query().Get("system") == "true"
	cfg := s.runner.Config()
	actions, err := executor.ListActions(cfg.Actions, cfg.Timeout, s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if actions == nil {
		actions = []executor.ActionDef{}
	}

	filtered := make([]executor.ActionDef, 0)
	for _, a := range actions {
		sys := strings.HasPrefix(a.ID, "@system/")
		if isSystem == sys {
			filtered = append(filtered, a)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}

func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	envVars := os.Environ()
	sensitiveSuffixes := []string{"key", "secret", "password", "token", "auth", "credential", "passwd"}
	env := make([]map[string]string, 0)
	for _, e := range envVars {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}
		lower := strings.ToLower(parts[0])
		redacted := false
		for _, s := range sensitiveSuffixes {
			if strings.Contains(lower, s) {
				redacted = true
				break
			}
		}
		val := parts[1]
		if redacted && val != "" {
			val = "***redacted***"
		}
		env = append(env, map[string]string{"name": parts[0], "value": val})
	}
	sort.Slice(env, func(i, j int) bool {
		return env[i]["name"] < env[j]["name"]
	})

	w.Header().Set("Content-Type", "application/json")
	cfg := s.runner.Config()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"version":    runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"uptime":     time.Since(s.startTime).String(),
		"goroutines": runtime.NumGoroutine(),
		"cpus":       runtime.NumCPU(),
		"config": map[string]interface{}{
			"host":        cfg.Host,
			"port":        cfg.Port,
			"timeout":     cfg.Timeout,
			"data_dir":    cfg.DataDir,
			"log_dir":     cfg.LogDir,
			"clean_days":  cfg.CleanDays,
			"max_log_num": cfg.MaxLogNum,
		},
		"environment": env,
	})
}

func (s *Server) handleClean(w http.ResponseWriter, r *http.Request) {
	id, err := s.runner.RunAction(r.Context(), s.db, s.runner.Config().LogDir, "@system/clean-logs", "")
	if err != nil {
		if errors.Is(err, executor.ErrPaused) {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"history_id": id})
}

var ErrReloadInProgress = errors.New("reload already in progress")

type reloadResult struct {
	Status     string   `json:"status"`
	Added      []string `json:"added"`
	Removed    []string `json:"removed"`
	Updated    []string `json:"updated"`
	Backfilled []string `json:"backfilled"`
	Warning    string   `json:"warning,omitempty"`
}

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	res, err := s.reload()
	if err != nil {
		if errors.Is(err, ErrReloadInProgress) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

// reload re-reads the config file and applies it gracefully: the new config
// is validated first, then new runs are paused, in-flight runs are drained,
// the new actions are swapped in, the scheduler is re-synced, runs resume,
// and ticks missed during the pause are backfilled with one catch-up run
// each. Only changes under "actions" take effect; anything else is reported
// in the response warning and needs a restart. Unchanged scheduler entries
// are left untouched, so scheduling resumes where it paused. On any failure
// the previous config keeps serving.
func (s *Server) reload() (*reloadResult, error) {
	if !s.reloadMu.TryLock() {
		return nil, ErrReloadInProgress
	}
	defer s.reloadMu.Unlock()

	path := s.cfg.ConfigPath
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("config file not found: %s, keeping current config", path)
	}
	next, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	cur := s.runner.Config()

	res := diffActions(cur.Actions, next.Actions)
	if changed := nonActionChanges(cur, next); len(changed) > 0 {
		res.Warning = fmt.Sprintf("restart required for: %s", strings.Join(changed, ", "))
	}
	if len(res.Added) == 0 && len(res.Removed) == 0 && len(res.Updated) == 0 {
		return res, nil
	}

	// Merge: only actions are hot-loaded, everything else keeps serving
	// the current values until restart.
	merged := *cur
	merged.Actions = next.Actions

	s.runner.Pause()
	s.runner.WaitIdle()
	s.runner.SwapConfig(&merged)
	syncErr := s.sched.Sync()
	s.runner.Resume()
	if syncErr != nil {
		return nil, syncErr
	}
	res.Backfilled = s.sched.Backfill()
	return res, nil
}

// nonActionChanges returns the names of config fields outside "actions"
// that differ between cur and next.
func nonActionChanges(cur, next *config.Config) []string {
	var changed []string
	if cur.Host != next.Host {
		changed = append(changed, "host")
	}
	if cur.Port != next.Port {
		changed = append(changed, "port")
	}
	if !reflect.DeepEqual(cur.Env, next.Env) {
		changed = append(changed, "env")
	}
	if cur.Timeout != next.Timeout {
		changed = append(changed, "timeout")
	}
	if cur.DataDir != next.DataDir {
		changed = append(changed, "data_dir")
	}
	if cur.CleanDays != next.CleanDays {
		changed = append(changed, "clean_days")
	}
	if cur.MaxLogNum != next.MaxLogNum {
		changed = append(changed, "max_log_num")
	}
	return changed
}

func diffActions(old, next map[string]*config.ActionConfig) *reloadResult {
	res := &reloadResult{Status: "ok", Added: []string{}, Removed: []string{}, Updated: []string{}, Backfilled: []string{}}
	for id, n := range next {
		o, ok := old[id]
		if !ok {
			res.Added = append(res.Added, id)
		} else if !actionConfigsEqual(o, n) {
			res.Updated = append(res.Updated, id)
		}
	}
	for id := range old {
		if _, ok := next[id]; !ok {
			res.Removed = append(res.Removed, id)
		}
	}
	sort.Strings(res.Added)
	sort.Strings(res.Removed)
	sort.Strings(res.Updated)
	return res
}

func actionConfigsEqual(a, b *config.ActionConfig) bool {
	if a == nil || b == nil {
		return a == b
	}
	return reflect.DeepEqual(*a, *b)
}
