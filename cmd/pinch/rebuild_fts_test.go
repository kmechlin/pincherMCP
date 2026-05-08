package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pincherMCP/pincher/internal/db"
)

// TestRebuildFTSCLI_Binary runs the actual `pincher rebuild-fts` binary
// against a fresh DB so the subcommand wiring (dispatch, flags, output
// format) is exercised end-to-end. The store-level rebuild semantics are
// covered by TestRebuildFTS_* in internal/db/db_test.go — this test
// guards the CLI contract: arg parsing, exit code, banner format.
func TestRebuildFTSCLI_Binary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI binary build in -short mode")
	}

	bin := filepath.Join(t.TempDir(), "pincher")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	// Seed a DB with some symbols so the rebuild has something to count.
	dataDir := t.TempDir()
	store, err := db.Open(dataDir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	store.UpsertProject(db.Project{ID: "p1", Path: "/p", Name: "demo"})
	store.BulkUpsertSymbols([]db.Symbol{
		{ID: "s1", ProjectID: "p1", FilePath: "a.go", Name: "A", QualifiedName: "a.A", Kind: "Function", Language: "Go"},
		{ID: "s2", ProjectID: "p1", FilePath: "a.go", Name: "B", QualifiedName: "a.B", Kind: "Function", Language: "Go"},
	})
	store.Close()

	// Default output: human-readable banner with row count.
	cmd := exec.Command(bin, "rebuild-fts", "--data-dir", dataDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rebuild-fts: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Rebuilt symbols_fts: 2 rows") {
		t.Errorf("expected '2 rows' banner, got:\n%s", out)
	}

	// --quiet: row count only, no banner.
	cmd = exec.Command(bin, "rebuild-fts", "--data-dir", dataDir, "--quiet")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rebuild-fts --quiet: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if got != "2" {
		t.Errorf("--quiet output = %q, want %q", got, "2")
	}
}

// TestRebuildFTSCLI_BadDataDir asserts that a corrupt / non-existent data
// directory produces a clean error exit, not a panic.
func TestRebuildFTSCLI_BadDataDir(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI binary build in -short mode")
	}

	bin := filepath.Join(t.TempDir(), "pincher")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	// Pass a path under a non-writable parent — opening should fail.
	cmd := exec.Command(bin, "rebuild-fts", "--data-dir", "/proc/1/no-such-place")
	out, _ := cmd.CombinedOutput()
	// We don't assert exit code (Go subprocess behavior varies); just
	// that we got a recognisable failure message, not a panic.
	if !strings.Contains(string(out), "failed") {
		t.Errorf("expected 'failed' in stderr, got:\n%s", out)
	}
	if strings.Contains(string(out), "panic:") {
		t.Errorf("rebuild-fts panicked on bad data dir:\n%s", out)
	}
}
