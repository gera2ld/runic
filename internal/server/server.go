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
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"runic/internal/config"
	"runic/internal/db"
	"runic/internal/executor"
	"runic/internal/logmgr"
)

//go:embed web/index.html
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
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/logs/", s.handleLogs)
	mux.HandleFunc("/api/actions/", s.handleActions)
	mux.HandleFunc("/api/actions", s.handleActions)
	mux.HandleFunc("/api/clean", s.handleClean)
	mux.HandleFunc("/api/system", s.handleSystem)

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
	data, err := uiContent.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "UI not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

const defaultHistoryLimit = 500

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("ids")
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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/logs/")
	if idStr == "" {
		http.Error(w, "missing log id", http.StatusBadRequest)
		return
	}
	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
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

var actionTriggerRe = regexp.MustCompile(`^/api/actions/([^/]+)/trigger$`)

func (s *Server) handleActions(w http.ResponseWriter, r *http.Request) {
	if matches := actionTriggerRe.FindStringSubmatch(r.URL.Path); matches != nil {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.triggerAction(w, r, matches[1])
		return
	}

	if r.URL.Path == "/api/actions" || r.URL.Path == "/api/actions/" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.listActions(w, r)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) triggerAction(w http.ResponseWriter, r *http.Request, actionID string) {
	actionID = strings.TrimSpace(actionID)
	if actionID == "" {
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

	historyID, err := s.runner.RunAction(context.Background(), s.db, s.cfg.LogDir, s.cfg.ActionDir, actionID, payload)
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
		"action_id":  actionID,
		"history_id": historyID,
	})
}

func (s *Server) listActions(w http.ResponseWriter, r *http.Request) {
	actions, err := executor.ListActions(s.cfg.ActionDir, s.cfg.Timeout, s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if actions == nil {
		actions = []executor.ActionDef{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(actions)
}

func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := logmgr.Clean(s.cfg.LogDir, s.db, s.cfg.CleanDays, s.cfg.MaxLogNum); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cleaned"})
}
