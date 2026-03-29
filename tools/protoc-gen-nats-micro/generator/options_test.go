package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	natspb "github.com/franchb/protoc-gen-nats-micro/tools/protoc-gen-nats-micro/nats/micro"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestGetServiceAndEndpointOptionsExtractNewMicroControls(t *testing.T) {
	file := buildTestFile(t, []*descriptorpb.DescriptorProto{
		messageDescriptor("CreateBlobRequest", stringField("id", 1)),
		messageDescriptor("CreateBlobResponse", stringField("id", 1)),
	}, []*descriptorpb.MethodDescriptorProto{
		methodDescriptor("CreateBlob", "CreateBlobRequest", "CreateBlobResponse", false, false, nil),
	})

	serviceOpts := &descriptorpb.ServiceOptions{}
	proto.SetExtension(serviceOpts, natspb.E_Service, &natspb.ServiceOptions{
		SubjectPrefix:      "blob",
		QueueGroupDisabled: true,
	})
	file.Service[0].Options = serviceOpts

	methodOpts := &descriptorpb.MethodOptions{}
	proto.SetExtension(methodOpts, natspb.E_Endpoint, &natspb.EndpointOptions{
		QueueGroupDisabled: true,
		PendingMsgLimit:    16,
		PendingBytesLimit:  2048,
	})
	file.Service[0].Method[0].Options = methodOpts

	_, target := newTestPlugin(t, file)

	service := target.Services[0]
	gotService := GetServiceOptions(service)
	if !gotService.QueueGroupDisabled {
		t.Fatal("GetServiceOptions() did not extract queue_group_disabled")
	}

	gotEndpoint := GetEndpointOptions(service.Methods[0])
	if !gotEndpoint.QueueGroupDisabled {
		t.Fatal("GetEndpointOptions() did not extract endpoint queue_group_disabled")
	}
	if gotEndpoint.PendingMsgLimit != 16 {
		t.Fatalf("GetEndpointOptions().PendingMsgLimit = %d, want 16", gotEndpoint.PendingMsgLimit)
	}
	if gotEndpoint.PendingBytesLimit != 2048 {
		t.Fatalf("GetEndpointOptions().PendingBytesLimit = %d, want 2048", gotEndpoint.PendingBytesLimit)
	}
}

func TestGenerateFileEmitsNewMicroControlWiring(t *testing.T) {
	file := buildTestFile(t, []*descriptorpb.DescriptorProto{
		messageDescriptor("CreateBlobRequest", stringField("id", 1)),
		messageDescriptor("CreateBlobResponse", stringField("id", 1)),
	}, []*descriptorpb.MethodDescriptorProto{
		methodDescriptor("CreateBlob", "CreateBlobRequest", "CreateBlobResponse", false, false, nil),
	})

	serviceOpts := &descriptorpb.ServiceOptions{}
	proto.SetExtension(serviceOpts, natspb.E_Service, &natspb.ServiceOptions{
		SubjectPrefix:      "blob",
		QueueGroupDisabled: true,
	})
	file.Service[0].Options = serviceOpts

	methodOpts := &descriptorpb.MethodOptions{}
	proto.SetExtension(methodOpts, natspb.E_Endpoint, &natspb.EndpointOptions{
		QueueGroupDisabled: true,
		PendingMsgLimit:    16,
		PendingBytesLimit:  2048,
	})
	file.Service[0].Method[0].Options = methodOpts

	gen, target := newTestPlugin(t, file)
	lang := NewGoLanguage()

	shared := gen.NewGeneratedFile("test/shared_nats.pb.go", target.GoImportPath)
	if err := lang.GenerateShared(shared, target); err != nil {
		t.Fatalf("GenerateShared() error = %v", err)
	}
	if err := GenerateFile(gen, target, lang); err != nil {
		t.Fatalf("GenerateFile() error = %v", err)
	}

	var mainFile string
	for _, f := range gen.Response().File {
		if strings.HasSuffix(f.GetName(), "_nats.pb.go") &&
			!strings.HasSuffix(f.GetName(), "shared_nats.pb.go") &&
			!strings.HasSuffix(f.GetName(), "_chunked_nats.pb.go") &&
			!strings.HasSuffix(f.GetName(), "_chunked_protoopaque_nats.pb.go") {
			mainFile = f.GetContent()
			break
		}
	}
	if mainFile == "" {
		t.Fatal("failed to find generated Go service file")
	}

	snippets := []string{
		"micro.WithGroupQueueGroupDisabled()",
		"micro.WithEndpointQueueGroupDisabled()",
		"pendingMsgLimit:    16",
		"pendingBytesLimit:  2048",
		"micro.WithEndpointPendingLimits(regCfg.pendingMsgLimit, regCfg.pendingBytesLimit)",
	}
	for _, snippet := range snippets {
		if !strings.Contains(mainFile, snippet) {
			t.Fatalf("generated file missing snippet %q", snippet)
		}
	}
}

func TestGenerateFileImportsOSForServiceWarnings(t *testing.T) {
	file := buildTestFile(t, []*descriptorpb.DescriptorProto{
		messageDescriptor("CreateBlobRequest", stringField("id", 1)),
		messageDescriptor("CreateBlobResponse", stringField("id", 1)),
	}, []*descriptorpb.MethodDescriptorProto{
		methodDescriptor("CreateBlob", "CreateBlobRequest", "CreateBlobResponse", false, false, nil),
	})

	gen, target := newTestPlugin(t, file)
	lang := NewGoLanguage()

	if err := GenerateFile(gen, target, lang); err != nil {
		t.Fatalf("GenerateFile() error = %v", err)
	}

	var mainFile string
	for _, f := range gen.Response().File {
		if strings.HasSuffix(f.GetName(), "_nats.pb.go") &&
			!strings.HasSuffix(f.GetName(), "shared_nats.pb.go") &&
			!strings.HasSuffix(f.GetName(), "_chunked_nats.pb.go") &&
			!strings.HasSuffix(f.GetName(), "_chunked_protoopaque_nats.pb.go") {
			mainFile = f.GetContent()
			break
		}
	}
	if mainFile == "" {
		t.Fatal("failed to find generated Go service file")
	}
	if !strings.Contains(mainFile, "\"os\"") {
		t.Fatalf("generated service file is missing os import:\n%s", mainFile)
	}
}

func TestOptionsProtoExposesNewKVFields(t *testing.T) {
	root := repoRootFromTest(t)
	optionsProtoPath := filepath.Join(root, "extensions", "proto", "natsmicro", "options.proto")

	data, err := os.ReadFile(optionsProtoPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", optionsProtoPath, err)
	}

	text := string(data)
	for _, required := range []string{
		"map<string, string> metadata = 7;",
		"google.protobuf.Duration limit_marker_ttl = 8;",
		"google.protobuf.Duration key_ttl = 9;",
		"google.protobuf.Duration purge_ttl = 10;",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("options.proto is missing new KV field %q", required)
		}
	}
}

func TestGetEndpointOptionsExtractsNewKVOptions(t *testing.T) {
	file := buildTestFile(t, []*descriptorpb.DescriptorProto{
		messageDescriptor("CreateBlobRequest", stringField("id", 1)),
		messageDescriptor("CreateBlobResponse", stringField("id", 1)),
	}, []*descriptorpb.MethodDescriptorProto{
		methodDescriptor("CreateBlob", "CreateBlobRequest", "CreateBlobResponse", false, false, nil),
	})

	methodOpts := &descriptorpb.MethodOptions{}
	proto.SetExtension(methodOpts, natspb.E_KvStore, &natspb.KVStoreOptions{
		Bucket:         "blob_cache",
		KeyTemplate:    "blob.{id}",
		Description:    "blob cache",
		MaxHistory:     8,
		ClientOnly:     true,
		Metadata:       map[string]string{"tier": "gold", "owner": "franchb"},
		LimitMarkerTtl: durationpb.New(15 * time.Minute),
		KeyTtl:         durationpb.New(5 * time.Minute),
		PurgeTtl:       durationpb.New(30 * time.Minute),
	})
	file.Service[0].Method[0].Options = methodOpts

	_, target := newTestPlugin(t, file)

	gotKV := GetEndpointOptions(target.Services[0].Methods[0]).KVStore
	if gotKV == nil {
		t.Fatal("GetEndpointOptions() did not extract kv_store")
	}
	if gotKV.Metadata["tier"] != "gold" || gotKV.Metadata["owner"] != "franchb" {
		t.Fatalf("GetEndpointOptions().KVStore.Metadata = %#v, want tier/owner entries", gotKV.Metadata)
	}
	if gotKV.LimitMarkerTTL != 15*time.Minute {
		t.Fatalf("GetEndpointOptions().KVStore.LimitMarkerTTL = %v, want %v", gotKV.LimitMarkerTTL, 15*time.Minute)
	}
	if gotKV.KeyTTL != 5*time.Minute {
		t.Fatalf("GetEndpointOptions().KVStore.KeyTTL = %v, want %v", gotKV.KeyTTL, 5*time.Minute)
	}
	if gotKV.PurgeTTL != 30*time.Minute {
		t.Fatalf("GetEndpointOptions().KVStore.PurgeTTL = %v, want %v", gotKV.PurgeTTL, 30*time.Minute)
	}
}

func TestGenerateFileEmitsNewKVFeatureWiring(t *testing.T) {
	file := buildTestFile(t, []*descriptorpb.DescriptorProto{
		messageDescriptor("CreateBlobRequest", stringField("id", 1)),
		messageDescriptor("CreateBlobResponse", stringField("id", 1)),
	}, []*descriptorpb.MethodDescriptorProto{
		methodDescriptor("CreateBlob", "CreateBlobRequest", "CreateBlobResponse", false, false, nil),
	})

	methodOpts := &descriptorpb.MethodOptions{}
	proto.SetExtension(methodOpts, natspb.E_KvStore, &natspb.KVStoreOptions{
		Bucket:         "blob_cache",
		KeyTemplate:    "blob.{id}",
		Description:    "blob cache",
		Metadata:       map[string]string{"tier": "gold"},
		LimitMarkerTtl: durationpb.New(15 * time.Minute),
		KeyTtl:         durationpb.New(5 * time.Minute),
		PurgeTtl:       durationpb.New(30 * time.Minute),
	})
	file.Service[0].Method[0].Options = methodOpts

	gen, target := newTestPlugin(t, file)
	lang := NewGoLanguage()

	shared := gen.NewGeneratedFile("test/shared_nats.pb.go", target.GoImportPath)
	if err := lang.GenerateShared(shared, target); err != nil {
		t.Fatalf("GenerateShared() error = %v", err)
	}
	if err := GenerateFile(gen, target, lang); err != nil {
		t.Fatalf("GenerateFile() error = %v", err)
	}

	var mainFile string
	for _, f := range gen.Response().File {
		if strings.HasSuffix(f.GetName(), "_nats.pb.go") &&
			!strings.HasSuffix(f.GetName(), "shared_nats.pb.go") &&
			!strings.HasSuffix(f.GetName(), "_chunked_nats.pb.go") &&
			!strings.HasSuffix(f.GetName(), "_chunked_protoopaque_nats.pb.go") {
			mainFile = f.GetContent()
		}
	}
	if mainFile == "" {
		t.Fatal("failed to find generated Go main file")
	}

	for _, snippet := range []string{
		"LimitMarkerTTL: 900000000000 * time.Nanosecond",
		"Metadata: map[string]string{",
		"\"tier\": \"gold\",",
		"func putBlobServiceKVValue(ctx context.Context, kv jetstream.KeyValue, key string, data []byte, keyTTL time.Duration) error",
		"jetstream.KeyTTL(keyTTL)",
		"putBlobServiceKVValue(ctx, kv, kvKey, data,",
		"300000000000*time.Nanosecond",
	} {
		if !strings.Contains(mainFile, snippet) {
			t.Fatalf("generated file missing snippet %q", snippet)
		}
	}

	for _, snippet := range []string{
		"func (c *BlobServiceNatsClient) PurgeCreateBlobFromKV(ctx context.Context, key string) error",
		"jetstream.PurgeTTL(1800000000000*time.Nanosecond)",
		"func putBlobServiceClientKVValue(ctx context.Context, kv jetstream.KeyValue, key string, data []byte, keyTTL time.Duration) error",
		"jetstream.KeyTTL(keyTTL)",
		"putBlobServiceClientKVValue(ctx, kv, key, data,",
		"300000000000*time.Nanosecond",
	} {
		if !strings.Contains(mainFile, snippet) {
			t.Fatalf("generated file missing snippet %q", snippet)
		}
	}
}
