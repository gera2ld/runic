package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadValidConfig(t *testing.T) {
	path := writeConfig(t, `
host: 127.0.0.1
port: 1337
timeout: 10
data_dir: .
clean_days: 30
max_log_num: 100
env:
  LANG: en_US.UTF-8
actions:
  deploy:
    name: Deploy
    timeout: 60
    command: ./scripts/deploy.sh
    cwd: /tmp
    cron: "*/5 * * * *"
    concurrency: 2
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected valid config to load, got: %v", err)
	}
	if cfg.Port != "1337" {
		t.Fatalf("expected port 1337, got %q", cfg.Port)
	}
	a := cfg.Actions["deploy"]
	if a == nil {
		t.Fatal("expected action 'deploy' to be loaded")
	}
	if a.Command != "./scripts/deploy.sh" || a.Cron != "*/5 * * * *" || *a.Concurrency != 2 {
		t.Fatalf("unexpected action definition: %+v", a)
	}
}

func TestLoadExampleConfig(t *testing.T) {
	// The shipped example must always pass validation.
	if _, err := Load("../../config.example.yml"); err != nil {
		t.Fatalf("config.example.yml failed validation: %v", err)
	}
}

func TestLoadRejects(t *testing.T) {
	cases := map[string]string{
		"unknown top-level field": "host: 127.0.0.1\nbogus: 1\n",
		"unknown action field":    "actions:\n  a:\n    command: echo hi\n    commad: echo typo\n",
		"missing command":         "actions:\n  a:\n    cron: '* * * * *'\n",
		"empty command":           "actions:\n  a:\n    command: '   '\n",
		"bad cron":                "actions:\n  a:\n    command: echo hi\n    cron: not-a-cron\n",
		"bad concurrency type":    "actions:\n  a:\n    command: echo hi\n    concurrency: many\n",
		"bad timeout type":        "timeout: soon\n",
		"bad env value type":      "env:\n  FOO: 123\n",
		"reserved id":             "actions:\n  '@system/x':\n    command: echo hi\n",
		"bad port in file":        "port: abc\n",
		"port out of range":       "port: 99999\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeConfig(t, content)); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

func TestLoadRejectsBadPortFromEnv(t *testing.T) {
	t.Setenv("RUNIC_PORT", "abc")
	if _, err := Load(writeConfig(t, "host: 127.0.0.1\n")); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestLoadEmptyAndMissingFiles(t *testing.T) {
	cfg, err := Load(writeConfig(t, "# nothing here\n"))
	if err != nil {
		t.Fatalf("expected empty config to load with defaults, got: %v", err)
	}
	if cfg.Port != "1337" || cfg.Timeout != 10 || len(cfg.Actions) != 0 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}

	cfg, err = Load(filepath.Join(t.TempDir(), "does-not-exist.yml"))
	if err != nil {
		t.Fatalf("expected missing config to load with defaults, got: %v", err)
	}
	if cfg.Port != "1337" {
		t.Fatalf("expected default port, got %q", cfg.Port)
	}
}

func TestLoadErrorMentionsOffendingField(t *testing.T) {
	_, err := Load(writeConfig(t, "actions:\n  a:\n    cron: '* * * * *'\n"))
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "command") {
		t.Fatalf("expected error to mention 'command', got: %v", err)
	}
}
