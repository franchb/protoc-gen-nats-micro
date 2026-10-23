package generator

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBufWorkspaceUsesGoogleapisModule(t *testing.T) {
	root := repoRootFromTest(t)
	bufYAMLPath := filepath.Join(root, "buf.yaml")

	data, err := os.ReadFile(bufYAMLPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", bufYAMLPath, err)
	}
	text := string(data)
	if strings.Contains(text, "third_party/googleapis") {
		t.Fatalf("buf.yaml still lists vendored googleapis:\n%s", text)
	}
	if !strings.Contains(text, "buf.build/googleapis/googleapis") {
		t.Fatalf("buf.yaml does not depend on buf.build/googleapis/googleapis:\n%s", text)
	}
}

func TestBufLockExistsAndVendorTreeIsGone(t *testing.T) {
	root := repoRootFromTest(t)

	lock, err := os.ReadFile(filepath.Join(root, "buf.lock"))
	if err != nil {
		t.Fatalf("buf.lock missing: %v", err)
	}
	if !strings.Contains(string(lock), "buf.build/googleapis/googleapis") {
		t.Fatalf("buf.lock does not pin buf.build/googleapis/googleapis:\n%s", lock)
	}

	vendor := filepath.Join(root, "third_party", "googleapis")
	if _, err := os.Stat(vendor); !os.IsNotExist(err) {
		t.Fatalf("vendored googleapis still present at %s (stat err=%v)", vendor, err)
	}
}

func repoRootFromTest(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
