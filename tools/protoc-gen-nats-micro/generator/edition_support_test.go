package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestChunkedHelperMatchesGoAPILevel(t *testing.T) {
	if _, err := exec.LookPath("buf"); err != nil {
		t.Skip("buf not on PATH")
	}
	plugin := buildPlugin(t)
	opts := filepath.Join(repoRootFromTest(t), "extensions", "proto", "natsmicro", "options.proto")

	opaque := generateChunked(t, plugin, opts, `edition = "2026";
package fixture.v1;
import "google/protobuf/go_features.proto";
import "natsmicro/options.proto";
option features.(pb.go).api_level = API_OPAQUE;
option go_package = "example.com/fixture/v1;fixturev1";
service Blob {
  rpc Upload(stream Chunk) returns (Done) {
    option (natsmicro.chunked_io) = { chunk_field: "data" };
  }
}
message Chunk { bytes data = 1; }
message Done { string id = 1; }
`)
	if _, err := os.Stat(filepath.Join(opaque, "gen", "service_chunked_protoopaque_nats.pb.go")); !os.IsNotExist(err) {
		t.Fatal("opaque file must not emit the protoopaque build-tag twin")
	}
	body := mustRead(t, filepath.Join(opaque, "gen", "service_chunked_nats.pb.go"))
	if strings.Contains(body, "go:build") {
		t.Fatalf("opaque chunked helper must be untagged:\n%s", body)
	}
	if !strings.Contains(body, "SetData(") {
		t.Fatalf("opaque chunked helper must call SetData:\n%s", body)
	}

	open := generateChunked(t, plugin, opts, `syntax = "proto3";
package fixture.v1;
import "natsmicro/options.proto";
option go_package = "example.com/fixture/v1;fixturev1";
service Blob {
  rpc Upload(stream Chunk) returns (Done) {
    option (natsmicro.chunked_io) = { chunk_field: "data" };
  }
}
message Chunk { bytes data = 1; }
message Done { string id = 1; }
`)
	openBody := mustRead(t, filepath.Join(open, "gen", "service_chunked_nats.pb.go"))
	tagged := mustRead(t, filepath.Join(open, "gen", "service_chunked_protoopaque_nats.pb.go"))
	if !strings.Contains(openBody, "!protoopaque") {
		t.Fatalf("proto3 helper must keep the !protoopaque tag:\n%s", openBody)
	}
	if !strings.Contains(tagged, "protoopaque") || !strings.Contains(tagged, "SetData(") {
		t.Fatalf("proto3 must still emit the tagged setter twin:\n%s", tagged)
	}
}

func generateChunked(t *testing.T, plugin, optionsProto, service string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "natsmicro"), 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(optionsProto)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "natsmicro", "options.proto"), string(src))
	writeFile(t, filepath.Join(root, "buf.yaml"), "version: v2\n")
	writeFile(t, filepath.Join(root, "buf.gen.yaml"), "version: v2\nplugins:\n  - local: "+plugin+"\n    out: gen\n    opt:\n      - paths=source_relative\n      - language=go\n")
	writeFile(t, filepath.Join(root, "service.proto"), service)
	cmd := exec.Command("buf", "generate")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("buf generate: %v\n%s", err, out)
	}
	return root
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
