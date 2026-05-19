package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceBinary(t *testing.T) {
	// Create a temp source file
	src, err := os.CreateTemp("", "test-src-*")
	if err != nil {
		t.Fatal(err)
	}
	srcPath := src.Name()
	defer os.Remove(srcPath)
	src.WriteString("new content")
	src.Close()

	if err := os.Chmod(srcPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a temp destination file
	dstDir, err := os.MkdirTemp("", "test-dst-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dstDir)
	dstPath := filepath.Join(dstDir, "runic")
	if err := os.WriteFile(dstPath, []byte("old content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test replaceBinary
	if err := replaceBinary(srcPath, dstPath); err != nil {
		t.Fatalf("replaceBinary failed: %v", err)
	}

	// Verify content
	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new content" {
		t.Errorf("expected 'new content', got %q", string(content))
	}

	// Verify permissions (0755)
	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("expected 0755, got %o", info.Mode().Perm())
	}

	// Verify src is removed
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Errorf("expected src to be gone, but it exists")
	}
}
