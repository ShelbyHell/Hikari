package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNodeJoinRaw(t *testing.T) {
	t.Setenv("MIKAN_NODE_JOIN", "")

	// 1. When no env and nonexistent file, empty
	t.Setenv("MIKAN_NODE_JOIN_FILE", "/nonexistent/secrets/node_join")
	if got := nodeJoinRaw(); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}

	// 2. When file exists, read trimmed content
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "node_join")
	if err := os.WriteFile(keyPath, []byte("  secret-join-key-456\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MIKAN_NODE_JOIN_FILE", keyPath)
	if got := nodeJoinRaw(); got != "secret-join-key-456" {
		t.Fatalf("got %q, want %q", got, "secret-join-key-456")
	}

	// 3. When MIKAN_NODE_JOIN is set, it takes priority
	t.Setenv("MIKAN_NODE_JOIN", "env-join-key")
	if got := nodeJoinRaw(); got != "env-join-key" {
		t.Fatalf("got %q, want %q", got, "env-join-key")
	}
}
