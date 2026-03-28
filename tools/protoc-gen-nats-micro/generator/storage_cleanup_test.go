package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"
)

func TestOptionsProtoDoesNotExposeGeneratedStorageAnnotations(t *testing.T) {
	root := repoRootFromTest(t)
	optionsProtoPath := filepath.Join(root, "extensions", "proto", "natsmicro", "options.proto")

	data, err := os.ReadFile(optionsProtoPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", optionsProtoPath, err)
	}

	text := string(data)
	for _, forbidden := range []string{
		"message KVStoreOptions",
		"message ObjectStoreOptions",
		"KVStoreOptions kv_store = 50003;",
		"ObjectStoreOptions object_store = 50004;",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("options.proto still exposes generated storage surface %q", forbidden)
		}
	}
}

func TestGenerateSharedOmitsJetStreamStorageHooks(t *testing.T) {
	file := buildTestFile(t, []*descriptorpb.DescriptorProto{
		messageDescriptor("DownloadRequest", stringField("id", 1)),
		messageDescriptor("SnapshotChunk", bytesField("data", 1)),
	}, []*descriptorpb.MethodDescriptorProto{
		methodDescriptor("Download", "DownloadRequest", "SnapshotChunk", false, true, nil),
	})

	gen, target := newTestPlugin(t, file)
	lang := NewGoLanguage()

	shared := gen.NewGeneratedFile("test/shared_nats.pb.go", target.GoImportPath)
	if err := lang.GenerateShared(shared, target); err != nil {
		t.Fatalf("GenerateShared() error = %v", err)
	}

	var sharedFile string
	for _, f := range gen.Response().File {
		if strings.HasSuffix(f.GetName(), "shared_nats.pb.go") {
			sharedFile = f.GetContent()
			break
		}
	}
	if sharedFile == "" {
		t.Fatal("failed to find generated shared Go file")
	}

	for _, forbidden := range []string{
		"jetstream.JetStream",
		"WithJetStream(",
		"WithNatsClientJetStream(",
		"KV/ObjectStore",
	} {
		if strings.Contains(sharedFile, forbidden) {
			t.Fatalf("generated shared Go file still contains storage hook %q:\n%s", forbidden, sharedFile)
		}
	}
}
