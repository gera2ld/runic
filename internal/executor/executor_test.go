package executor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"runic/internal/config"
	"runic/internal/db"
)

func TestListActionsPopulatesNextRunAndConcurrency(t *testing.T) {
	dir := t.TempDir()

	actionPath := filepath.Join(dir, "sample.yml")
	if err := os.WriteFile(actionPath, []byte(`
command: echo hi
cron: "* * * * *"
concurrency: 0
`), 0644); err != nil {
		t.Fatal(err)
	}

	actions, err := ListActions(dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Concurrency == nil || *actions[0].Concurrency != 0 {
		t.Fatalf("expected concurrency 0, got %#v", actions[0].Concurrency)
	}
	if actions[0].NextRun == nil {
		t.Fatal("expected next_run to be populated")
	}
	if !actions[0].NextRun.After(time.Now()) {
		t.Fatalf("expected next_run to be in the future, got %v", actions[0].NextRun)
	}
}

func TestRunActionEnforcesConcurrencyLimit(t *testing.T) {
	root := t.TempDir()
	actionDir := filepath.Join(root, "actions")
	logDir := filepath.Join(root, "logs")
	if err := os.MkdirAll(actionDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}

	actionPath := filepath.Join(actionDir, "slow.yml")
	if err := os.WriteFile(actionPath, []byte(`
command: sleep 1
concurrency: 1
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{Timeout: 5}
	d, err := db.Open(filepath.Join(root, "runic.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	runner := NewRunner(cfg)
	historyID, err := runner.RunAction(context.Background(), d, logDir, actionDir, "slow", "")
	if err != nil {
		t.Fatal(err)
	}
	if historyID == 0 {
		t.Fatal("expected a history ID")
	}

	_, err = runner.RunAction(context.Background(), d, logDir, actionDir, "slow", "")
	if err == nil {
		t.Fatal("expected concurrency limit error")
	}
	if err != nil && !errors.Is(err, ErrConcurrencyLimitReached) {
		t.Fatalf("expected concurrency limit error, got %v", err)
	}

	time.Sleep(1300 * time.Millisecond)
}
