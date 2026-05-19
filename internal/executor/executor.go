package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"runic/internal/config"
	"runic/internal/db"
	"runic/internal/logmgr"
	"gopkg.in/yaml.v3"
)

type ActionDef struct {
	ID      string `yaml:"-" json:"id"`
	Name    string `yaml:"name" json:"name"`
	Timeout int    `yaml:"timeout" json:"timeout"`
	Command string `yaml:"command" json:"command"`
	Cwd     string `yaml:"cwd" json:"cwd"`
}

type Runner struct {
	cfg *config.Config
}

func NewRunner(cfg *config.Config) *Runner {
	return &Runner{cfg: cfg}
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
	if def.Name == "" {
		def.Name = actionID
	}
	return &def, nil
}

type RunResult struct {
	HistoryID int64
	ActionID  string
	Status    string
	ExitCode  int
	Duration  int64
}

func (r *Runner) RunAction(ctx context.Context, d *db.DB, logDir, actionDir, actionID, payload string) (int64, error) {
	def, err := LoadAction(actionDir, actionID)
	if err != nil {
		return 0, err
	}

	sl, err := logmgr.NewStreamLogger(logDir, actionID)
	if err != nil {
		return 0, fmt.Errorf("failed to create stream logger: %w", err)
	}

	historyID, err := d.InsertHistory(actionID, sl.FilePath())
	if err != nil {
		sl.Close()
		return 0, fmt.Errorf("failed to insert history: %w", err)
	}

	go func() {
		defer sl.Close()
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

func ListActions(actionDir string) ([]ActionDef, error) {
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
			continue
		}
		actions = append(actions, *def)
	}
	return actions, nil
}
