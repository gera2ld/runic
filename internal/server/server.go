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
	"runtime"
	"sort"
	"strconv"
	"strings"
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

	os.MkdirAll(cfg.ActionDir, 0755)

	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("GET /api/logs/{hid}", s.handleLogs)
	mux.HandleFunc("GET /api/actions", s.handleListActions)
	mux.HandleFunc("GET /api/actions/{id}", s.handleGetAction)
	mux.HandleFunc("POST /api/actions/{id}/trigger", s.handleTriggerAction)
	mux.HandleFunc("POST /api/clean", s.handleClean)
	mux.HandleFunc("GET /api/system", s.handleSystem)

	fmt.Printf("[server] listening on %s:%s\n", cfg.Host, cfg.Port)
	srv := &http.Server{
		Addr:         cfg.Host + ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "[server] fatal: %v\n", err)
		os.Exit(1)
	}
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
	def, err := executor.LoadAction(s.cfg.ActionDir, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	executor.NormalizeAction(def, s.cfg.Timeout)
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

	historyID, err := s.runner.RunAction(context.Background(), s.db, s.cfg.LogDir, s.cfg.ActionDir, id, payload)
	if err != nil {
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
	actions, err := executor.ListActions(s.cfg.ActionDir, s.cfg.Timeout, s.db)
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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"version":   runtime.Version(),
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
		"uptime":    time.Since(s.startTime).String(),
		"goroutines": runtime.NumGoroutine(),
		"cpus":      runtime.NumCPU(),
		"config": map[string]interface{}{
			"host":        s.cfg.Host,
			"port":        s.cfg.Port,
			"timeout":     s.cfg.Timeout,
			"data_dir":    s.cfg.DataDir,
			"log_dir":     s.cfg.LogDir,
			"action_dir":  s.cfg.ActionDir,
			"clean_days":  s.cfg.CleanDays,
			"max_log_num": s.cfg.MaxLogNum,
		},
		"environment": env,
	})
}

func (s *Server) handleClean(w http.ResponseWriter, r *http.Request) {
	id, err := s.runner.RunAction(r.Context(), s.db, s.cfg.LogDir, s.cfg.ActionDir, "@system/clean-logs", "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"history_id": id})
}
