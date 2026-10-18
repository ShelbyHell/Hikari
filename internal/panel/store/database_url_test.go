package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDatabaseURLSecret(t *testing.T) {
	t.Setenv("MIKAN_DATABASE_URL", "")

	// 1. When no env and no file, error
	t.Setenv("MIKAN_DATABASE_PASSWORD_FILE", "/nonexistent/secrets/db_password")
	if _, err := DatabaseURL(); err != errNoDatabaseURL {
		t.Fatalf("expected errNoDatabaseURL, got %v", err)
	}

	// 2. When password file exists, construct DSN
	tmpDir := t.TempDir()
	pwPath := filepath.Join(tmpDir, "db_password")
	if err := os.WriteFile(pwPath, []byte("super-secret-password-123\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MIKAN_DATABASE_PASSWORD_FILE", pwPath)
	dsn, err := DatabaseURL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "postgresql://mikan:super-secret-password-123@localhost/mikan?host=/run/postgresql"
	if dsn != want {
		t.Fatalf("got %q, want %q", dsn, want)
	}

	// 3. When MIKAN_DATABASE_URL is set, it overrides the password file
	t.Setenv("MIKAN_DATABASE_URL", "postgresql://custom:url@host/db")
	dsn, err = DatabaseURL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dsn != "postgresql://custom:url@host/db" {
		t.Fatalf("got %q, want %q", dsn, "postgresql://custom:url@host/db")
	}
}
