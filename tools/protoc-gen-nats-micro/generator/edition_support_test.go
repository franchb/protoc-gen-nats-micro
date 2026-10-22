package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBufGenerateAcceptsEdition2026AndProto3(t *testing.T) {
	if _, err := exec.LookPath("buf"); err != nil {
		t.Skip("buf not on PATH")
	}

	plugin := buildPlugin(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "buf.yaml"), "version: v2\n")
	writeFile(t, filepath.Join(root, "buf.gen.yaml"), `version: v2
plugins:
  - local: `+plugin+`
    out: gen
    opt:
      - paths=source_relative
      - language=go
`)
	writeFile(t, filepath.Join(root, "edition2026.proto"), `edition = "2026";

package fixture.v1;

import "google/protobuf/go_features.proto";

option go_package = "example.com/fixture/v1;fixturev1";

option features.(pb.go).api_level = API_OPAQUE;

service Echo {
  rpc Say(SayRequest) returns (SayResponse);
}
message SayRequest { string text = 1; }
message SayResponse { string text = 1; }
`)
	writeFile(t, filepath.Join(root, "legacy.proto"), `syntax = "proto3";

package fixture.v1;

option go_package = "example.com/fixture/v1;fixturev1";

service Legacy {
  rpc Ping(PingRequest) returns (PingResponse);
}
message PingRequest { string text = 1; }
message PingResponse { string text = 1; }
`)

	cmd := exec.Command("buf", "generate")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("buf generate: %v\n%s", err, out)
	}
}

func buildPlugin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "protoc-gen-nats-micro")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Join(repoRootFromTest(t), "tools", "protoc-gen-nats-micro")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build plugin: %v\n%s", err, out)
	}
	return bin
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
