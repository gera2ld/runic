package executor

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
	"runic/internal/config"
	"runic/internal/db"
	"runic/internal/logmgr"
)

type ActionDef struct {
	ID            string     `yaml:"-" json:"id"`
	Name          string     `yaml:"name" json:"name"`
	Timeout       int        `yaml:"timeout" json:"timeout"`
	Command       string     `yaml:"command" json:"command"`
	Cwd           string     `yaml:"cwd" json:"cwd"`
	Cron          string     `yaml:"cron" json:"cron"`
	Concurrency   *int       `yaml:"concurrency" json:"concurrency"`
	NextRun       *time.Time `yaml:"-" json:"next_run,omitempty"`
	LastRun       *time.Time `yaml:"-" json:"last_run,omitempty"`
	LastRunStatus string     `yaml:"-" json:"last_run_status,omitempty"`
}

type Runner struct {
	cfg    atomic.Pointer[config.Config]
	db     *db.DB
	mu     sync.Mutex
	active map[string]int
	paused bool
	wg     sync.WaitGroup
}

func NewRunner(cfg *config.Config, d *db.DB) *Runner {
	r := &Runner{db: d, active: make(map[string]int)}
	r.cfg.Store(cfg)
	return r
}

// Config returns the currently active config.
func (r *Runner) Config() *config.Config {
	return r.cfg.Load()
}

// SwapConfig replaces the active config. Callers must pause the runner and
// drain in-flight runs first; see Pause and WaitIdle.
func (r *Runner) SwapConfig(cfg *config.Config) {
	r.cfg.Store(cfg)
}

// Pause rejects new runs until Resume is called.
func (r *Runner) Pause() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.paused = true
}

// Resume accepts new runs again.
func (r *Runner) Resume() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.paused = false
}

// WaitIdle blocks until all in-flight runs have settled. It must be called
// after Pause, so no new runs can start while waiting.
func (r *Runner) WaitIdle() {
	r.wg.Wait()
}

func NormalizeAction(def *ActionDef, defaultTimeout int) {
	if def.Name == "" {
		def.Name = def.ID
	}
	if def.Timeout <= 0 {
		def.Timeout = defaultTimeout
	}
	if def.Cwd == "" {
		def.Cwd = "."
	} else {
		def.Cwd = os.ExpandEnv(def.Cwd)
	}
	if def.Concurrency == nil {
		concurrency := 1
		def.Concurrency = &concurrency
	} else if *def.Concurrency < 0 {
		concurrency := 1
		def.Concurrency = &concurrency
	}
}

//go:embed system_actions/*.yml
var systemActionsFS embed.FS

var SystemActions = make(map[string]ActionDef)

func init() {
	entries, err := systemActionsFS.ReadDir("system_actions")
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		data, err := systemActionsFS.ReadFile("system_actions/" + e.Name())
		if err != nil {
			panic(err)
		}
		var def ActionDef
		if err := yaml.Unmarshal(data, &def); err != nil {
			panic(err)
		}
		id := "@system/" + strings.TrimSuffix(e.Name(), ".yml")
		def.ID = id
		SystemActions[id] = def
	}
}

func LoadAction(actions map[string]*config.ActionConfig, actionID string) (*ActionDef, error) {
	var sysDef *ActionDef
	if sys, ok := SystemActions[actionID]; ok {
		sysDef = &sys
	}

	// Try loading from the single config file
	if raw, ok := actions[actionID]; ok && raw != nil {
		def := ActionDef{
			Name:        raw.Name,
			Timeout:     raw.Timeout,
			Command:     raw.Command,
			Cwd:         raw.Cwd,
			Cron:        raw.Cron,
			Concurrency: raw.Concurrency,
		}
		def.ID = actionID
		if sysDef != nil {
			// System actions have read-only commands
			def.Command = sysDef.Command
		}
		if def.Command == "" {
			return nil, fmt.Errorf("action %q has no command", actionID)
		}
		def.Cwd = os.ExpandEnv(def.Cwd)
		return &def, nil
	}

	// Try loading from system actions
	if sysDef != nil {
		copy := *sysDef
		return &copy, nil
	}

	return nil, fmt.Errorf("action %q not found", actionID)
}

func (r *Runner) acquire(actionID string, limit int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.paused {
		return fmt.Errorf("%w: %s", ErrPaused, actionID)
	}
	if limit > 0 {
		if r.active[actionID] >= limit {
			return fmt.Errorf("%w: %s", ErrConcurrencyLimitReached, actionID)
		}
		r.active[actionID]++
	}
	r.wg.Add(1)
	return nil
}

func (r *Runner) release(actionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.active[actionID] <= 1 {
		delete(r.active, actionID)
	} else {
		r.active[actionID]--
	}
	r.wg.Done()
}

type RunResult struct {
	HistoryID int64
	ActionID  string
	Status    string
	ExitCode  int
	Duration  int64
}

var ErrConcurrencyLimitReached = errors.New("action concurrency limit reached")

var ErrPaused = errors.New("server is paused for reload, try again later")

func (r *Runner) RunAction(ctx context.Context, d *db.DB, logDir, actionID, payload string) (int64, error) {
	cfg := r.Config()
	def, err := LoadAction(cfg.Actions, actionID)
	if err != nil {
		return 0, err
	}
	NormalizeAction(def, cfg.Timeout)
	if err := r.acquire(actionID, *def.Concurrency); err != nil {
		return 0, err
	}

	sl, err := logmgr.NewStreamLogger(logDir, actionID)
	if err != nil {
		r.release(actionID)
		return 0, fmt.Errorf("failed to create stream logger: %w", err)
	}

	historyID, err := d.InsertHistory(actionID, sl.FilePath())
	if err != nil {
		sl.Close()
		r.release(actionID)
		return 0, fmt.Errorf("failed to insert history: %w", err)
	}

	go func() {
		defer sl.Close()
		defer r.release(actionID)
		var result RunResult
		if strings.HasPrefix(def.Command, "@internal:") {
			result = r.runInternalCommand(ctx, def, sl, actionID)
		} else {
			result = r.runCommand(ctx, def, sl, payload, actionID)
		}
		result.HistoryID = historyID
		d.UpdateHistory(result.HistoryID, result.Status, result.Duration)
		fmt.Printf("[executor] action=%s status=%s duration=%dms exit_code=%d\n",
			actionID, result.Status, result.Duration, result.ExitCode)
	}()

	return historyID, nil
}

func (r *Runner) runInternalCommand(ctx context.Context, def *ActionDef, sl *logmgr.StreamLogger, actionID string) RunResult {
	start := time.Now()
	var err error
	var status string
	var exitCode int

	cmd := strings.TrimPrefix(def.Command, "@internal:")
	switch cmd {
	case "clean-logs":
		cfg := r.Config()
		fmt.Fprintf(sl.Writer(), "[system] starting log cleanup (days=%d, max_logs=%d)\n", cfg.CleanDays, cfg.MaxLogNum)
		err = logmgr.Clean(cfg.LogDir, r.db, cfg.CleanDays, cfg.MaxLogNum)
		if err == nil {
			fmt.Fprintf(sl.Writer(), "[system] log cleanup completed\n")
		}
	default:
		err = fmt.Errorf("unknown internal command: %s", cmd)
	}

	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		fmt.Fprintf(sl.Writer(), "[system] error: %v\n", err)
		status = "FAILED"
		exitCode = 1
	} else {
		status = "SUCCESS"
		exitCode = 0
	}

	return RunResult{
		ActionID: actionID,
		Status:   status,
		ExitCode: exitCode,
		Duration: elapsed,
	}
}

func (r *Runner) runCommand(ctx context.Context, def *ActionDef, sl *logmgr.StreamLogger, payload, actionID string) RunResult {
	cfg := r.Config()
	timeout := time.Duration(def.Timeout) * time.Second
	if timeout <= 0 {
		timeout = time.Duration(cfg.Timeout) * time.Second
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cwd := def.Cwd
	if cwd == "" {
		cwd = "."
	}

	cmd := exec.CommandContext(runCtx, "bash", "-c", def.Command)
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Env = append(cmd.Env, fmt.Sprintf("RUNIC_ACTION_ID=%s", actionID))
	cmd.Env = append(cmd.Env, fmt.Sprintf("RUNIC_ACTION_NAME=%s", def.Name))
	if payload != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("RUNIC_PAYLOAD=%s", payload))
	}
	cmd.Stdout = sl.Writer()
	cmd.Stderr = sl.Writer()

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start).Milliseconds()

	var status string
	exitCode := 0
	if err != nil {
		fmt.Fprintf(os.Stderr, "[executor] action=%s cmd error: %v\n", actionID, err)
		if runCtx.Err() == context.DeadlineExceeded {
			status = "TIMEOUT"
			exitCode = 124
		} else {
			status = "FAILED"
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}
	} else {
		status = "SUCCESS"
	}

	return RunResult{
		ActionID: actionID,
		Status:   status,
		ExitCode: exitCode,
		Duration: elapsed,
	}
}

func ListActions(actions map[string]*config.ActionConfig, defaultTimeout int, d *db.DB) ([]ActionDef, error) {
	actionsMap := make(map[string]ActionDef)

	// Load system actions first
	for id, sys := range SystemActions {
		def := sys
		NormalizeAction(&def, defaultTimeout)
		actionsMap[id] = def
	}

	// Load from the single config file, possibly overriding system actions
	for id := range actions {
		def, err := LoadAction(actions, id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[executor] failed to load action %s: %v\n", id, err)
			continue
		}
		NormalizeAction(def, defaultTimeout)
		actionsMap[id] = *def
	}

	var list []ActionDef
	for id, def := range actionsMap {
		if def.Cron != "" {
			if schedule, err := cron.ParseStandard(def.Cron); err == nil {
				nextRun := schedule.Next(time.Now())
				def.NextRun = &nextRun
			}
		}
		if d != nil {
			lastRun, err := d.GetLatestHistoryByActionID(id)
			if err == nil && lastRun != nil {
				def.LastRun = &lastRun.CreatedAt
				def.LastRunStatus = lastRun.Status
			}
		}
		list = append(list, def)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})
	return list, nil
}
