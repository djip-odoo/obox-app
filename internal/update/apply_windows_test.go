//go:build windows

package update

import (
	"path/filepath"
	"testing"
)

func TestApply_MissingFile(t *testing.T) {
	nonExistent := filepath.Join(t.TempDir(), "nonexistent-installer.exe")
	err := Apply(nonExistent)
	if err == nil {
		t.Errorf("expected error for missing installer file, got nil")
	}
}

func TestApply_EmptyPath(t *testing.T) {
	err := Apply("")
	if err == nil {
		t.Errorf("expected error for empty installer path, got nil")
	}
}
