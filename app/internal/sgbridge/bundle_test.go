//go:build windows

package sgbridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractEmbeddedBundle(t *testing.T) {
	if !hasEmbeddedBundle() {
		t.Fatal("expected hasEmbeddedBundle() to be true on Windows")
	}

	tmpDir, err := os.MkdirTemp("", "sg_bundle_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := extractEmbeddedBundle(tmpDir); err != nil {
		t.Fatalf("extractEmbeddedBundle failed: %v", err)
	}

	if !bundleComplete(tmpDir) {
		t.Fatalf("bundleComplete returned false for extracted bundle in %s", tmpDir)
	}

	// Verify crucial files exist
	expectedFiles := []string{
		filepath.Join("prebuild", "win32-x64", "security_guard.node"),
		filepath.Join("resources", "x64", "AliSafeProxy.dll"),
		filepath.Join("resources", "x64", "ps", "02ec4c71f5479b186c370314aa93c4ea"),
		filepath.Join("resources", "x64", "ps", "09c4452bf247fbedd32cbdd84ad66e33"),
		filepath.Join("resources", "x64", "ps", "fb3a2752f31b33d0cded64f2d398d25f"),
		"sg_daemon.js",
	}

	for _, rel := range expectedFiles {
		p := filepath.Join(tmpDir, rel)
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("expected file %s missing: %v", rel, err)
		} else if info.Size() == 0 {
			t.Errorf("file %s is unexpectedly 0 bytes", rel)
		}
	}
}

func TestEnsureBundleWithEmbedded(t *testing.T) {
	tmpDataDir, err := os.MkdirTemp("", "sg_ensure_bundle_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDataDir)

	// ensureBundle extracts to filepath.Join(tmpDataDir, "security-guard")
	bundleDir, err := ensureBundle(tmpDataDir)
	if err != nil {
		t.Fatalf("ensureBundle failed: %v", err)
	}

	if !bundleComplete(bundleDir) {
		t.Fatalf("ensureBundle did not return a complete bundle at %s", bundleDir)
	}
}
