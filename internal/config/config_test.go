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

func TestLoadServerTags(t *testing.T) {
	// No tags anywhere: default to wildcard, same as pre-tags behavior.
	cfg, err := Load(writeConfig(t, "timeout: 10\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Tags) != 1 || cfg.Tags[0] != "*" {
		t.Fatalf("expected default server tags [*], got %v", cfg.Tags)
	}

	// Tags from the config file.
	cfg, err = Load(writeConfig(t, "timeout: 10\ntags:\n  - prod\n  - gpu\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Tags) != 2 || cfg.Tags[0] != "prod" || cfg.Tags[1] != "gpu" {
		t.Fatalf("expected file server tags [prod gpu], got %v", cfg.Tags)
	}

	// RUNIC_TAGS overrides the file, comma-separated.
	t.Setenv("RUNIC_TAGS", "prod, gpu ,,")
	cfg, err = Load(writeConfig(t, "timeout: 10\ntags:\n  - other\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Tags) != 2 || cfg.Tags[0] != "prod" || cfg.Tags[1] != "gpu" {
		t.Fatalf("expected env server tags [prod gpu], got %v", cfg.Tags)
	}
}

func TestLoadDisableAllTags(t *testing.T) {
	t.Setenv("RUNIC_TAGS", "-")
	cfg, err := Load(writeConfig(t, "timeout: 10\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Tags) != 1 || cfg.Tags[0] != "-" {
		t.Fatalf("expected server tags [-], got %v", cfg.Tags)
	}
}

func TestLoadActionTags(t *testing.T) {
	path := writeConfig(t, `
actions:
  w:
    command: echo w
    tags: "*"
  s:
    command: echo s
    tags: prod
  l:
    command: echo l
    tags: [prod, gpu]
  u:
    command: echo u
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := []string(cfg.Actions["w"].Tags); len(got) != 1 || got[0] != "*" {
		t.Fatalf("expected [*], got %v", got)
	}
	if got := []string(cfg.Actions["s"].Tags); len(got) != 1 || got[0] != "prod" {
		t.Fatalf("expected [prod], got %v", got)
	}
	if got := []string(cfg.Actions["l"].Tags); len(got) != 2 || got[0] != "prod" || got[1] != "gpu" {
		t.Fatalf("expected [prod gpu], got %v", got)
	}
	if len(cfg.Actions["u"].Tags) != 0 {
		t.Fatalf("expected no tags, got %v", cfg.Actions["u"].Tags)
	}
}

func TestLoadExampleConfig(t *testing.T) { // The shipped example must always pass validation.
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
