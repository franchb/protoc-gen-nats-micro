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

func TestChunkedHelperFollowsInputMessageAPI(t *testing.T) {
	if _, err := exec.LookPath("buf"); err != nil {
		t.Skip("buf not on PATH")
	}
	plugin := buildPlugin(t)
	opts := filepath.Join(repoRootFromTest(t), "extensions", "proto", "natsmicro", "options.proto")

	// Opaque service file, open chunk message defined elsewhere.
	openInput := generateChunkedFiles(t, plugin, opts, map[string]string{
		"chunk.proto": `syntax = "proto3";
package fixture.v1;
option go_package = "example.com/fixture/v1;fixturev1";
message OpenChunk { bytes data = 1; }
`,
		"service.proto": `edition = "2026";
package fixture.v1;
import "google/protobuf/go_features.proto";
import "natsmicro/options.proto";
import "chunk.proto";
option features.(pb.go).api_level = API_OPAQUE;
option go_package = "example.com/fixture/v1;fixturev1";
service Blob {
  rpc Upload(stream OpenChunk) returns (Done) {
    option (natsmicro.chunked_io) = { chunk_field: "data" };
  }
}
message Done { string id = 1; }
`,
	})
	openBody := mustRead(t, filepath.Join(openInput, "gen", "service_chunked_nats.pb.go"))
	if !strings.Contains(openBody, "!protoopaque") || strings.Contains(openBody, "SetData(") {
		t.Fatalf("open input must use the composite-literal file:\n%s", openBody)
	}
	tagged := mustRead(t, filepath.Join(openInput, "gen", "service_chunked_protoopaque_nats.pb.go"))
	if !strings.Contains(tagged, "SetData(") {
		t.Fatalf("open input must keep the protoopaque setter twin:\n%s", tagged)
	}
	if _, err := os.Stat(filepath.Join(openInput, "gen", "service_chunked_opaque_nats.pb.go")); !os.IsNotExist(err) {
		t.Fatal("open-only input must not emit the mixed opaque file")
	}

	// Proto3 service file, opaque chunk message defined elsewhere.
	opaqueInput := generateChunkedFiles(t, plugin, opts, map[string]string{
		"chunk.proto": `edition = "2026";
package fixture.v1;
import "google/protobuf/go_features.proto";
option features.(pb.go).api_level = API_OPAQUE;
option go_package = "example.com/fixture/v1;fixturev1";
export message OpaqueChunk { bytes data = 1; }
`,
		"service.proto": `syntax = "proto3";
package fixture.v1;
import "natsmicro/options.proto";
import "chunk.proto";
option go_package = "example.com/fixture/v1;fixturev1";
service Blob {
  rpc Upload(stream OpaqueChunk) returns (Done) {
    option (natsmicro.chunked_io) = { chunk_field: "data" };
  }
}
message Done { string id = 1; }
`,
	})
	opaqueBody := mustRead(t, filepath.Join(opaqueInput, "gen", "service_chunked_nats.pb.go"))
	if strings.Contains(opaqueBody, "go:build") || !strings.Contains(opaqueBody, "SetData(") {
		t.Fatalf("opaque input must be an untagged setter:\n%s", opaqueBody)
	}
	if _, err := os.Stat(filepath.Join(opaqueInput, "gen", "service_chunked_protoopaque_nats.pb.go")); !os.IsNotExist(err) {
		t.Fatal("opaque-only input must not emit the protoopaque twin")
	}

	// One file, three message-level APIs. Edition 2024 with no option is opaque.
	mixed := generateChunked(t, plugin, opts, `edition = "2026";
package fixture.v1;
import "google/protobuf/go_features.proto";
import "natsmicro/options.proto";
option features.(pb.go).api_level = API_OPAQUE;
option go_package = "example.com/fixture/v1;fixturev1";
service Blob {
  rpc UploadOpen(stream OpenChunk) returns (Done) {
    option (natsmicro.chunked_io) = { chunk_field: "data" };
  }
  rpc UploadHybrid(stream HybridChunk) returns (Done) {
    option (natsmicro.chunked_io) = { chunk_field: "data" };
  }
  rpc UploadOpaque(stream OpaqueChunk) returns (Done) {
    option (natsmicro.chunked_io) = { chunk_field: "data" };
  }
  rpc UploadEdition(stream EditionChunk) returns (Done) {
    option (natsmicro.chunked_io) = { chunk_field: "data" };
  }
}
message OpenChunk {
  option features.(pb.go).api_level = API_OPEN;
  bytes data = 1;
}
message HybridChunk {
  option features.(pb.go).api_level = API_HYBRID;
  bytes data = 1;
}
message OpaqueChunk {
  option features.(pb.go).api_level = API_OPAQUE;
  bytes data = 1;
}
message EditionChunk { bytes data = 1; }
message Done { string id = 1; }
`)
	// EditionChunk has no message override; the file is API_OPAQUE.
	// A separate edition 2024 file with no option is covered below.
	mixedOpen := mustRead(t, filepath.Join(mixed, "gen", "service_chunked_nats.pb.go"))
	mixedTagged := mustRead(t, filepath.Join(mixed, "gen", "service_chunked_protoopaque_nats.pb.go"))
	mixedOpaque := mustRead(t, filepath.Join(mixed, "gen", "service_chunked_opaque_nats.pb.go"))
	if !strings.Contains(mixedOpen, "!protoopaque") {
		t.Fatalf("mixed open file must keep !protoopaque:\n%s", mixedOpen)
	}
	for _, name := range []string{"UploadOpen", "UploadHybrid"} {
		if !strings.Contains(mixedOpen, name) || !strings.Contains(mixedTagged, name) {
			t.Fatalf("%s must be in the build-tagged pair", name)
		}
		if strings.Contains(mixedOpaque, name) {
			t.Fatalf("%s must not be in the untagged opaque file:\n%s", name, mixedOpaque)
		}
	}
	for _, name := range []string{"UploadOpaque", "UploadEdition"} {
		if !strings.Contains(mixedOpaque, name) || !strings.Contains(mixedOpaque, "SetData(") {
			t.Fatalf("%s must be an untagged setter:\n%s", name, mixedOpaque)
		}
		if strings.Contains(mixedOpen, name) || strings.Contains(mixedTagged, name) {
			t.Fatalf("%s must not be in the build-tagged pair", name)
		}
	}
	if strings.Contains(mixedOpaque, "go:build") {
		t.Fatalf("mixed opaque file must be untagged:\n%s", mixedOpaque)
	}

	edition2024 := generateChunked(t, plugin, opts, `edition = "2024";
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
	editionBody := mustRead(t, filepath.Join(edition2024, "gen", "service_chunked_nats.pb.go"))
	if strings.Contains(editionBody, "go:build") || !strings.Contains(editionBody, "SetData(") {
		t.Fatalf("edition 2024 with no api_level must be an untagged setter:\n%s", editionBody)
	}
}

func generateChunked(t *testing.T, plugin, optionsProto, service string) string {
	t.Helper()
	return generateChunkedFiles(t, plugin, optionsProto, map[string]string{"service.proto": service})
}

func generateChunkedFiles(t *testing.T, plugin, optionsProto string, files map[string]string) string {
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
	for name, body := range files {
		writeFile(t, filepath.Join(root, name), body)
	}
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
