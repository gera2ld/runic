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
