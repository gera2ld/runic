package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

//go:embed config.schema.json
var schemaJSON []byte

const schemaURL = "https://raw.githubusercontent.com/gera2ld/runic/main/internal/config/config.schema.json"

var compileSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	var doc any
	if err := json.Unmarshal(schemaJSON, &doc); err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(schemaURL, doc); err != nil {
		return nil, err
	}
	return c.Compile(schemaURL)
})

// Action ids must not be empty, contain whitespace, or start with '@'
// ('@' is reserved for system actions such as @system/clean-logs).
var actionIDPattern = regexp.MustCompile(`^[^\s@][^\s]*$`)

func validate(data []byte) error {
	sch, err := compileSchema()
	if err != nil {
		return err
	}
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw == nil {
		return nil // empty file: all defaults
	}
	return sch.Validate(raw)
}

func validateSemantics(cfg *Config) error {
	for id, a := range cfg.Actions {
		if a == nil {
			return fmt.Errorf("action %q: definition must be an object", id)
		}
		if !actionIDPattern.MatchString(id) {
			return fmt.Errorf("action %q: id must not be empty, contain whitespace, or start with '@' (reserved for system actions)", id)
		}
		if strings.TrimSpace(a.Command) == "" {
			return fmt.Errorf("action %q: command must not be empty", id)
		}
		if a.Cron != "" {
			if _, err := cron.ParseStandard(a.Cron); err != nil {
				return fmt.Errorf("action %q: invalid cron %q: %v", id, a.Cron, err)
			}
		}
	}
	return nil
}

type ActionConfig struct {
	Name        string `yaml:"name"`
	Timeout     int    `yaml:"timeout"`
	Command     string `yaml:"command"`
	Cwd         string `yaml:"cwd"`
	Cron        string `yaml:"cron"`
	Concurrency *int   `yaml:"concurrency"`
	Tags        TagSet `yaml:"tags"`
}

// TagSet is a list of tags an action is targeted at. It accepts a single
// string ("*"), a list of strings, or nothing (inactive everywhere).
// "*" matches every server.
type TagSet []string

func (t *TagSet) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err == nil {
		*t = TagSet{s}
		return nil
	}
	var list []string
	if err := value.Decode(&list); err != nil {
		return err
	}
	*t = TagSet(list)
	return nil
}

type Config struct {
	Host           string                   `yaml:"host"`
	Port           string                   `yaml:"port"`
	Env            map[string]string        `yaml:"env"`
	Timeout        int                      `yaml:"timeout"`
	DataDir        string                   `yaml:"data_dir"`
	Actions        map[string]*ActionConfig `yaml:"actions"`
	Tags           []string                 `yaml:"tags"`
	ConfigPath     string                   `yaml:"-"`
	DataDirFromEnv bool                     `yaml:"-"`
	DBPath         string                   `yaml:"-"`
	LogDir         string                   `yaml:"-"`
	CleanDays      int                      `yaml:"clean_days"`
	MaxLogNum      int                      `yaml:"max_log_num"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = "config.yml"
	}
	cfg := &Config{
		Timeout:   10,
		Env:       make(map[string]string),
		Actions:   make(map[string]*ActionConfig),
		CleanDays: 30,
		MaxLogNum: 100,
	}
	cfg.ConfigPath = path

	data, err := os.ReadFile(path)
	if err == nil {
		if err := validate(data); err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
		if err := validateSemantics(cfg); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if cfg.Port == "" {
		if p := os.Getenv("RUNIC_PORT"); p != "" {
			cfg.Port = p
		} else {
			cfg.Port = "1337"
		}
	}
	if p, err := strconv.Atoi(cfg.Port); err != nil || p < 1 || p > 65535 {
		return nil, fmt.Errorf("invalid port %q: must be a number between 1 and 65535", cfg.Port)
	}
	if cfg.Host == "" {
		if h := os.Getenv("RUNIC_HOST"); h != "" {
			cfg.Host = h
		} else {
			cfg.Host = "127.0.0.1"
		}
	}
	if cfg.DataDir == "" {
		cfg.DataDir = os.Getenv("RUNIC_DATA_DIR")
		if cfg.DataDir != "" {
			cfg.DataDirFromEnv = true
		}
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "."
	} else if !cfg.DataDirFromEnv && !filepath.IsAbs(cfg.DataDir) {
		cfg.DataDir = filepath.Join(filepath.Dir(path), cfg.DataDir)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10
	}
	if tags, ok := os.LookupEnv("RUNIC_TAGS"); ok && strings.TrimSpace(tags) != "" {
		cfg.Tags = ParseTags(tags)
	}
	if len(cfg.Tags) == 0 {
		// No tags declared: run everything, same as before tags existed.
		cfg.Tags = []string{"*"}
	}

	cfg.DBPath = filepath.Join(cfg.DataDir, "runic.db")
	cfg.LogDir = filepath.Join(cfg.DataDir, "logs")

	return cfg, nil
}

// ParseTags splits a comma-separated tag list (e.g. RUNIC_TAGS) into
// trimmed, non-empty tags.
func ParseTags(s string) []string {
	var tags []string
	for _, t := range strings.Split(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}
