package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
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
	cfg    *config.Config
	db     *db.DB
	runner *executor.Runner
	sched  *executor.Scheduler
}

func Serve(cfg *config.Config, runner *executor.Runner, d *db.DB, sched *executor.Scheduler) {
	s := &Server{
		cfg:    cfg,
		db:     d,
		runner: runner,
		sched:  sched,
	}

	os.MkdirAll(cfg.ActionDir, 0755)

	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/logs/", s.handleLogs)
	mux.HandleFunc("/api/actions/", s.handleActions)
	mux.HandleFunc("/api/actions", s.handleActions)
	mux.HandleFunc("/api/clean", s.handleClean)

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
		entries, err = s.db.ListHistory(50)
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
	actions, err := executor.ListActions(s.cfg.ActionDir, s.cfg.Timeout)
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
