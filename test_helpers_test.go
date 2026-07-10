package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTempFile writes content to a temp file and returns its path.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test-file")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// removeTempFile removes a temp file (best effort).
func removeTempFile(t *testing.T, path string) {
	t.Helper()
	_ = os.Remove(path)
}
