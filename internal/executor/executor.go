package executor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
	"runic/internal/config"
	"runic/internal/db"
	"runic/internal/logmgr"
)

type ActionDef struct {
	ID          string     `yaml:"-" json:"id"`
	Name        string     `yaml:"name" json:"name"`
	Timeout     int        `yaml:"timeout" json:"timeout"`
	Command     string     `yaml:"command" json:"command"`
	Cwd         string     `yaml:"cwd" json:"cwd"`
	Cron        string     `yaml:"cron" json:"cron"`
	Concurrency *int       `yaml:"concurrency" json:"concurrency"`
	NextRun     *time.Time `yaml:"-" json:"next_run,omitempty"`
	LastRun     *time.Time `yaml:"-" json:"last_run,omitempty"`
	LastRunStatus string   `yaml:"-" json:"last_run_status,omitempty"`
}

type Runner struct {
	cfg    *config.Config
	mu     sync.Mutex
	active map[string]int
}

func NewRunner(cfg *config.Config) *Runner {
	return &Runner{cfg: cfg, active: make(map[string]int)}
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

func LoadAction(actionDir, actionID string) (*ActionDef, error) {
	data, err := os.ReadFile(actionDir + "/" + actionID + ".yml")
	if err != nil {
		return nil, fmt.Errorf("failed to read action file: %w", err)
	}
	var def ActionDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to parse action YAML: %w", err)
	}
	if def.Command == "" {
		return nil, fmt.Errorf("action %q has no command", actionID)
	}
	def.ID = actionID
	def.Cwd = os.ExpandEnv(def.Cwd)
	return &def, nil
}

func (r *Runner) tryAcquire(actionID string, limit int) bool {
	if limit <= 0 {
		return true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.active[actionID] >= limit {
		return false
	}
	r.active[actionID]++
	return true
}

func (r *Runner) release(actionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.active[actionID] <= 1 {
		delete(r.active, actionID)
		return
	}
	r.active[actionID]--
}

type RunResult struct {
	HistoryID int64
	ActionID  string
	Status    string
	ExitCode  int
	Duration  int64
}

var ErrConcurrencyLimitReached = errors.New("action concurrency limit reached")

func (r *Runner) RunAction(ctx context.Context, d *db.DB, logDir, actionDir, actionID, payload string) (int64, error) {
	def, err := LoadAction(actionDir, actionID)
	if err != nil {
		return 0, err
	}
	NormalizeAction(def, r.cfg.Timeout)
	if !r.tryAcquire(actionID, *def.Concurrency) {
		return 0, fmt.Errorf("%w: %s", ErrConcurrencyLimitReached, actionID)
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
		result := r.runCommand(ctx, def, sl, payload, actionID)
		result.HistoryID = historyID
		d.UpdateHistory(result.HistoryID, result.Status, result.Duration)
		fmt.Printf("[executor] action=%s status=%s duration=%dms exit_code=%d\n",
			actionID, result.Status, result.Duration, result.ExitCode)
	}()

	return historyID, nil
}

func (r *Runner) runCommand(ctx context.Context, def *ActionDef, sl *logmgr.StreamLogger, payload, actionID string) RunResult {
	timeout := time.Duration(def.Timeout) * time.Second
	if timeout <= 0 {
		timeout = time.Duration(r.cfg.Timeout) * time.Second
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
	for k, v := range r.cfg.Env {
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

func ListActions(actionDir string, defaultTimeout int, d *db.DB) ([]ActionDef, error) {
	entries, err := os.ReadDir(actionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var actions []ActionDef
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".yml")
		def, err := LoadAction(actionDir, id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[executor] failed to load action %s: %v\n", id, err)
			continue
		}
		NormalizeAction(def, defaultTimeout)
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
		actions = append(actions, *def)
	}
	return actions, nil
}
