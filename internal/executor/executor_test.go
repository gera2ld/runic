package executor

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"runic/internal/config"
	"runic/internal/db"
)

func TestListActionsPopulatesNextRunAndConcurrency(t *testing.T) {
	zero := 0
	actions := map[string]*config.ActionConfig{
		"sample": {
			Command:     "echo hi",
			Cron:        "* * * * *",
			Concurrency: &zero,
			Tags:        config.TagSet{"*"},
		},
	}

	got, err := ListActions(actions, []string{"prod"}, 10, nil)
	if err != nil {
		t.Fatal(err)
	}

	var action *ActionDef
	for _, a := range got {
		if a.ID == "sample" {
			action = &a
			break
		}
	}

	if action == nil {
		t.Fatal("expected action 'sample' not found")
	}
	if !action.Active {
		t.Fatal("expected action 'sample' to be active")
	}
	if action.Concurrency == nil || *action.Concurrency != 0 {
		t.Fatalf("expected concurrency 0, got %#v", action.Concurrency)
	}
	if action.NextRun == nil {
		t.Fatal("expected next_run to be populated")
	}
	if !action.NextRun.After(time.Now()) {
		t.Fatalf("expected next_run to be in the future, got %v", action.NextRun)
	}
}

func TestRunActionEnforcesConcurrencyLimit(t *testing.T) {
	root := t.TempDir()
	logDir := filepath.Join(root, "logs")

	one := 1
	cfg := &config.Config{
		Timeout: 5,
		LogDir:  logDir,
		Actions: map[string]*config.ActionConfig{
			"slow": {
				Command:     "sleep 1",
				Concurrency: &one,
			},
		},
	}
	d, err := db.Open(filepath.Join(root, "runic.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	runner := NewRunner(cfg, d)
	historyID, err := runner.RunAction(context.Background(), d, logDir, "slow", "")
	if err != nil {
		t.Fatal(err)
	}
	if historyID == 0 {
		t.Fatal("expected a history ID")
	}

	_, err = runner.RunAction(context.Background(), d, logDir, "slow", "")
	if err == nil {
		t.Fatal("expected concurrency limit error")
	}
	if err != nil && !errors.Is(err, ErrConcurrencyLimitReached) {
		t.Fatalf("expected concurrency limit error, got %v", err)
	}

	time.Sleep(1300 * time.Millisecond)
}

func TestPauseDrainAndResume(t *testing.T) {
	root := t.TempDir()
	logDir := filepath.Join(root, "logs")
	zero := 0
	cfg := &config.Config{
		Timeout: 10,
		LogDir:  logDir,
		Actions: map[string]*config.ActionConfig{
			"slow": {Command: "sleep 1", Concurrency: &zero},
		},
	}
	d, err := db.Open(filepath.Join(root, "runic.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	runner := NewRunner(cfg, d)
	if _, err := runner.RunAction(context.Background(), d, logDir, "slow", ""); err != nil {
		t.Fatal(err)
	}

	runner.Pause()
	if _, err := runner.RunAction(context.Background(), d, logDir, "slow", ""); !errors.Is(err, ErrPaused) {
		t.Fatalf("expected ErrPaused while paused, got %v", err)
	}

	drained := make(chan struct{})
	go func() {
		runner.WaitIdle()
		close(drained)
	}()
	select {
	case <-drained:
		t.Fatal("WaitIdle returned while a run was still in flight")
	case <-time.After(300 * time.Millisecond):
	}
	select {
	case <-drained:
	case <-time.After(10 * time.Second):
		t.Fatal("WaitIdle did not return after the run settled")
	}

	runner.Resume()
	if _, err := runner.RunAction(context.Background(), d, logDir, "slow", ""); err != nil {
		t.Fatalf("expected runs to be accepted after resume, got %v", err)
	}
	runner.WaitIdle()
}

func TestSchedulerBackfillsMissedTicks(t *testing.T) {
	root := t.TempDir()
	logDir := filepath.Join(root, "logs")
	cfg := &config.Config{
		Timeout: 5,
		LogDir:  logDir,
		Actions: map[string]*config.ActionConfig{
			"tick": {Command: "echo tick", Cron: "* * * * * *", Tags: config.TagSet{"*"}},
		},
	}
	d, err := db.Open(filepath.Join(root, "runic.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	runner := NewRunner(cfg, d)
	sched := &Scheduler{
		cron:    cron.New(cron.WithSeconds()),
		runner:  runner,
		db:      d,
		logDir:  logDir,
		entries: make(map[string]cron.EntryID),
		specs:   make(map[string]string),
		missed:  make(map[string]struct{}),
	}
	if err := sched.Sync(); err != nil {
		t.Fatal(err)
	}
	sched.cron.Start()
	defer sched.cron.Stop()

	runner.Pause()
	deadline := time.Now().Add(5 * time.Second)
	for {
		sched.mu.Lock()
		n := len(sched.missed)
		sched.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no tick was missed while paused")
		}
		time.Sleep(100 * time.Millisecond)
	}

	runner.Resume()
	backfilled := sched.Backfill()
	if len(backfilled) != 1 || backfilled[0] != "tick" {
		t.Fatalf("expected [tick] to be backfilled, got %v", backfilled)
	}

	deadline = time.Now().Add(5 * time.Second)
	for {
		latest, err := d.GetLatestHistoryByActionID("tick")
		if err == nil && latest != nil && latest.Status == "SUCCESS" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("backfilled run did not succeed (latest=%+v, err=%v)", latest, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestTagsMatch(t *testing.T) {
	cases := []struct {
		name       string
		actionTags []string
		serverTags []string
		want       bool
	}{
		{"wildcard action", []string{"*"}, []string{}, true},
		{"wildcard in list", []string{"prod", "*"}, []string{"other"}, true},
		{"no action tags", nil, []string{"prod"}, false},
		{"empty action tags", []string{}, []string{"prod"}, false},
		{"no server tags defaults to wildcard", []string{"prod"}, nil, true},
		{"matching tag", []string{"prod", "gpu"}, []string{"gpu"}, true},
		{"no matching tag", []string{"prod"}, []string{"gpu"}, false},
		{"wildcard server", []string{"prod"}, []string{"*"}, true},
		{"wildcard server, untagged action", nil, []string{"*"}, true},
		{"disabled server", []string{"*"}, []string{"-"}, false},
		{"disabled server, tagged action", []string{"prod"}, []string{"-"}, false},
		{"disabled server, untagged action", nil, []string{"-"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := TagsMatch(c.actionTags, c.serverTags); got != c.want {
				t.Fatalf("TagsMatch(%v, %v) = %v, want %v", c.actionTags, c.serverTags, got, c.want)
			}
		})
	}
}

func TestInactiveActionsAreSkipped(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		Timeout: 5,
		Tags:    []string{"prod"},
		Actions: map[string]*config.ActionConfig{
			"on":       {Command: "echo on", Cron: "* * * * *", Tags: config.TagSet{"prod"}},
			"off":      {Command: "echo off", Cron: "* * * * *", Tags: config.TagSet{"gpu"}},
			"untagged": {Command: "echo untagged", Cron: "* * * * *"},
		},
	}
	d, err := db.Open(filepath.Join(root, "runic.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	runner := NewRunner(cfg, d)
	sched := NewScheduler(runner, d, filepath.Join(root, "logs"))
	if err := sched.Sync(); err != nil {
		t.Fatal(err)
	}
	if _, ok := sched.entries["on"]; !ok {
		t.Fatal("expected active action 'on' to be scheduled")
	}
	if _, ok := sched.entries["off"]; ok {
		t.Fatal("expected inactive action 'off' not to be scheduled")
	}
	if _, ok := sched.entries["untagged"]; ok {
		t.Fatal("expected untagged action to never be scheduled")
	}

	ctx := context.Background()
	// A manual trigger always runs, even for inactive actions.
	historyID, err := runner.RunAction(ctx, d, filepath.Join(root, "logs"), "off", "")
	if err != nil {
		t.Fatalf("expected manual trigger of inactive action to run, got %v", err)
	}
	if historyID == 0 {
		t.Fatal("expected a history ID")
	}
	runner.WaitIdle()
}

func TestSyncPreservesUnchangedEntries(t *testing.T) {
	cfg := &config.Config{
		Timeout: 5,
		Actions: map[string]*config.ActionConfig{
			"keep": {Command: "echo keep", Cron: "* * * * *", Tags: config.TagSet{"*"}},
			"gone": {Command: "echo gone", Cron: "0 * * * *", Tags: config.TagSet{"*"}},
		},
	}
	runner := NewRunner(cfg, nil)
	sched := NewScheduler(runner, nil, t.TempDir())
	if err := sched.Sync(); err != nil {
		t.Fatal(err)
	}
	keepBefore := sched.entries["keep"]
	if keepBefore == 0 {
		t.Fatal("expected a cron entry for 'keep'")
	}

	runner.SwapConfig(&config.Config{
		Timeout: 5,
		Actions: map[string]*config.ActionConfig{
			"keep": {Command: "echo keep", Cron: "* * * * *", Tags: config.TagSet{"*"}},
			"new":  {Command: "echo new", Cron: "30 * * * *", Tags: config.TagSet{"*"}},
		},
	})
	if err := sched.Sync(); err != nil {
		t.Fatal(err)
	}
	// Unchanged entries keep their schedule, so scheduling resumes where
	// it paused instead of restarting.
	if sched.entries["keep"] != keepBefore {
		t.Fatal("expected the unchanged 'keep' entry to survive the re-sync")
	}
	if _, ok := sched.entries["new"]; !ok {
		t.Fatal("expected a new cron entry for 'new'")
	}
	if _, ok := sched.entries["gone"]; ok {
		t.Fatal("expected the removed 'gone' entry to be gone")
	}
}
