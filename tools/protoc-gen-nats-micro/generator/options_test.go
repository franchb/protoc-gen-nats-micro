package generator

import (
	"strings"
	"testing"

	natspb "github.com/franchb/protoc-gen-nats-micro/tools/protoc-gen-nats-micro/nats/micro"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
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
