package executor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandEnv(t *testing.T) {
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("skipping test; HOME environment variable not set")
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"/abs/path", "/abs/path"},
		{"./rel/path", "./rel/path"},
		{"$HOME", home},
		{"${HOME}", home},
		{"$HOME/foo", filepath.Join(home, "foo")},
		{"${HOME}/foo/bar", filepath.Join(home, "foo/bar")},
	}

	for _, tt := range tests {
		got := os.ExpandEnv(tt.input)
		if got != tt.expected {
			t.Errorf("os.ExpandEnv(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestNormalizeAction(t *testing.T) {
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("skipping test; HOME environment variable not set")
	}

	tests := []struct {
		name     string
		input    ActionDef
		expected ActionDef
	}{
		{
			name: "default values",
			input: ActionDef{ID: "test"},
			expected: ActionDef{ID: "test", Name: "test", Timeout: 30, Cwd: "."},
		},
		{
			name: "expand env in cwd",
			input: ActionDef{ID: "test", Cwd: "$HOME/foo"},
			expected: ActionDef{ID: "test", Name: "test", Timeout: 30, Cwd: filepath.Join(home, "foo")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := tt.input
			NormalizeAction(&def, 30)
			if def.Name != tt.expected.Name {
				t.Errorf("expected Name %q, got %q", tt.expected.Name, def.Name)
			}
			if def.Timeout != tt.expected.Timeout {
				t.Errorf("expected Timeout %d, got %d", tt.expected.Timeout, def.Timeout)
			}
			if def.Cwd != tt.expected.Cwd {
				t.Errorf("expected Cwd %q, got %q", tt.expected.Cwd, def.Cwd)
			}
		})
	}
}
