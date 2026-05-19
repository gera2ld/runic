package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Host    string            `yaml:"host"`
	Port    string            `yaml:"port"`
	Env     map[string]string `yaml:"env"`
	Timeout int               `yaml:"timeout"`
	DataDir string
	DBPath  string
	LogDir  string
	ActionDir string
	CleanDays int
	MaxLogNum int
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		Timeout:   10,
		Env:       make(map[string]string),
		CleanDays: 30,
		MaxLogNum: 100,
	}

	data, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
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
	if cfg.Host == "" {
		if h := os.Getenv("RUNIC_HOST"); h != "" {
			cfg.Host = h
		} else {
			cfg.Host = "127.0.0.1"
		}
	}
	if cfg.DataDir == "" {
		cfg.DataDir = os.Getenv("RUNIC_DATA_DIR")
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "."
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10
	}

	cfg.DBPath = filepath.Join(cfg.DataDir, "runic.db")
	cfg.LogDir = filepath.Join(cfg.DataDir, "logs")
	cfg.ActionDir = filepath.Join(cfg.DataDir, "actions")

	return cfg, nil
}
