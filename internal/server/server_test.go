package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"runic/internal/config"
	"runic/internal/db"
	"runic/internal/executor"
)

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func newTestServer(t *testing.T, path string) *Server {
	t.Helper()
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	d, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	runner := executor.NewRunner(cfg, d)
	sched := executor.NewScheduler(runner, d, cfg.LogDir)
	return &Server{cfg: cfg, db: d, runner: runner, sched: sched, startTime: time.Now()}
}

func postReload(t *testing.T, s *Server) (int, reloadResult) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/reload", nil)
	rec := httptest.NewRecorder()
	s.handler().ServeHTTP(rec, req)
	var res reloadResult
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatal(err)
		}
	}
	return rec.Code, res
}

func userActionIDs(t *testing.T, s *Server) []string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/actions", nil)
	rec := httptest.NewRecorder()
	s.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var actions []executor.ActionDef
	if err := json.Unmarshal(rec.Body.Bytes(), &actions); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(actions))
	for _, a := range actions {
		ids = append(ids, a.ID)
	}
	return ids
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestReloadAppliesActionChanges(t *testing.T) {
	path := writeTestConfig(t, "timeout: 10\nactions:\n  a:\n    command: echo one\n")
	s := newTestServer(t, path)
	if ids := userActionIDs(t, s); !equalStrings(ids, []string{"a"}) {
		t.Fatalf("expected [a], got %v", ids)
	}

	if err := os.WriteFile(path, []byte("timeout: 10\nactions:\n  b:\n    command: echo two\n"), 0644); err != nil {
		t.Fatal(err)
	}
	code, res := postReload(t, s)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if !equalStrings(res.Added, []string{"b"}) || !equalStrings(res.Removed, []string{"a"}) {
		t.Fatalf("unexpected diff: %+v", res)
	}
	if ids := userActionIDs(t, s); !equalStrings(ids, []string{"b"}) {
		t.Fatalf("expected [b] after reload, got %v", ids)
	}
}

func TestReloadWarnsOnNonActionChanges(t *testing.T) {
	path := writeTestConfig(t, "timeout: 10\nactions:\n  a:\n    command: echo one\n")
	s := newTestServer(t, path)

	// Non-actions changes no longer block the reload: actions still apply,
	// the live config keeps the old values, and the response warns about
	// what needs a restart.
	if err := os.WriteFile(path, []byte("timeout: 20\nactions:\n  a:\n    command: echo two\n"), 0644); err != nil {
		t.Fatal(err)
	}
	code, res := postReload(t, s)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if !strings.Contains(res.Warning, "timeout") {
		t.Fatalf("expected warning to mention timeout, got %q", res.Warning)
	}
	if !equalStrings(res.Updated, []string{"a"}) {
		t.Fatalf("expected actions to still apply, got %+v", res)
	}
	if got := s.runner.Config().Timeout; got != 10 {
		t.Fatalf("expected live timeout to stay 10 until restart, got %d", got)
	}
	def, err := executor.LoadAction(s.runner.Config().Actions, "a")
	if err != nil {
		t.Fatal(err)
	}
	if def.Command != "echo two" {
		t.Fatalf("expected new command to be live, got %q", def.Command)
	}
}

func TestReloadWarningOnlyAppliesNothing(t *testing.T) {
	path := writeTestConfig(t, "timeout: 10\nactions:\n  a:\n    command: echo one\n")
	s := newTestServer(t, path)

	if err := os.WriteFile(path, []byte("timeout: 20\nclean_days: 7\nactions:\n  a:\n    command: echo one\n"), 0644); err != nil {
		t.Fatal(err)
	}
	code, res := postReload(t, s)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if !strings.Contains(res.Warning, "timeout") || !strings.Contains(res.Warning, "clean_days") {
		t.Fatalf("expected warning to name the changed fields, got %q", res.Warning)
	}
	if len(res.Added) != 0 || len(res.Removed) != 0 || len(res.Updated) != 0 {
		t.Fatalf("expected no action changes, got %+v", res)
	}
	cfg := s.runner.Config()
	if cfg.Timeout != 10 || cfg.CleanDays != 30 {
		t.Fatalf("expected live values to be unchanged, got %+v", cfg)
	}
}

func TestReloadRejectsInvalidConfig(t *testing.T) {
	path := writeTestConfig(t, "timeout: 10\nactions:\n  a:\n    command: echo one\n")
	s := newTestServer(t, path)

	if err := os.WriteFile(path, []byte("timeout: 10\nactions:\n  a:\n    cron: '* * * * *'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	code, _ := postReload(t, s)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
	if ids := userActionIDs(t, s); !equalStrings(ids, []string{"a"}) {
		t.Fatalf("expected old config to keep serving [a], got %v", ids)
	}
}

func TestReloadMissingFile(t *testing.T) {
	path := writeTestConfig(t, "timeout: 10\nactions:\n  a:\n    command: echo one\n")
	s := newTestServer(t, path)

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if code, _ := postReload(t, s); code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
	if ids := userActionIDs(t, s); !equalStrings(ids, []string{"a"}) {
		t.Fatalf("expected old config to keep serving [a], got %v", ids)
	}
}

func TestReloadDrainsInFlightRuns(t *testing.T) {
	path := writeTestConfig(t, "timeout: 10\nactions:\n  slow:\n    command: sleep 2\n")
	s := newTestServer(t, path)

	req := httptest.NewRequest(http.MethodPost, "/api/actions/slow/trigger", nil)
	rec := httptest.NewRecorder()
	s.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := os.WriteFile(path, []byte("timeout: 10\nactions:\n  fast:\n    command: echo fast\n"), 0644); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	code, res := postReload(t, s)
	elapsed := time.Since(start)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if elapsed < 2*time.Second {
		t.Fatalf("expected reload to wait for the in-flight run, returned after %v", elapsed)
	}
	if !equalStrings(res.Added, []string{"fast"}) || !equalStrings(res.Removed, []string{"slow"}) {
		t.Fatalf("unexpected diff: %+v", res)
	}
	latest, err := s.db.GetLatestHistoryByActionID("slow")
	if err != nil || latest == nil || latest.Status != "SUCCESS" {
		t.Fatalf("expected the in-flight run to settle as SUCCESS, got %+v, %v", latest, err)
	}
	if ids := userActionIDs(t, s); !equalStrings(ids, []string{"fast"}) {
		t.Fatalf("expected [fast] after reload, got %v", ids)
	}
}

func TestTriggerWhilePausedReturns503(t *testing.T) {
	path := writeTestConfig(t, "timeout: 10\nactions:\n  a:\n    command: echo one\n")
	s := newTestServer(t, path)

	s.runner.Pause()
	defer s.runner.Resume()
	req := httptest.NewRequest(http.MethodPost, "/api/actions/a/trigger", nil)
	rec := httptest.NewRecorder()
	s.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "paused") {
		t.Fatalf("expected body to mention pause, got %q", rec.Body.String())
	}
}
