package generator

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRootFromTest(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func TestBufWorkspaceUsesVendoredGoogleAPI(t *testing.T) {
	root := repoRootFromTest(t)
	bufYAMLPath := filepath.Join(root, "buf.yaml")

	data, err := os.ReadFile(bufYAMLPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", bufYAMLPath, err)
	}

	text := string(data)
	if strings.Contains(text, "buf.build/googleapis/googleapis") {
		t.Fatalf("buf.yaml still references remote googleapis dependency:\n%s", text)
	}
	if !strings.Contains(text, "path: third_party/googleapis") {
		t.Fatalf("buf.yaml does not include vendored googleapis module:\n%s", text)
	}
}

func TestVendoredGoogleAPISubtreePresent(t *testing.T) {
	root := repoRootFromTest(t)

	requiredFiles := []string{
		filepath.Join(root, "third_party", "googleapis", "LICENSE"),
		filepath.Join(root, "third_party", "googleapis", "README.vendor.md"),
		filepath.Join(root, "third_party", "googleapis", "google", "api", "annotations.proto"),
		filepath.Join(root, "third_party", "googleapis", "google", "api", "http.proto"),
		filepath.Join(root, "third_party", "googleapis", "google", "api", "field_behavior.proto"),
	}

	for _, path := range requiredFiles {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("vendored googleapis file missing: %s (%v)", path, err)
		}
		if info.IsDir() {
			t.Fatalf("expected vendored file, got directory: %s", path)
		}
	}
}
