package generator

import (
	"strings"
	"testing"

	natspb "github.com/franchb/protoc-gen-nats-micro/tools/protoc-gen-nats-micro/nats/micro"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestGenerateFileRejectsChunkedIOOnUnaryMethod(t *testing.T) {
	file := buildTestFile(t, []*descriptorpb.DescriptorProto{
		messageDescriptor("UploadRequest", bytesField("data", 1)),
		messageDescriptor("UploadResponse", bytesField("data", 1)),
	}, []*descriptorpb.MethodDescriptorProto{
		methodDescriptor("Upload", "UploadRequest", "UploadResponse", false, false, &natspb.ChunkedIOOptions{
			ChunkField:       "data",
			DefaultChunkSize: 65536,
		}),
	})

	gen, target := newTestPlugin(t, file)
	lang := NewGoLanguage()

	err := GenerateFile(gen, target, lang)
	if err == nil {
		t.Fatal("GenerateFile() succeeded, want unary chunked_io validation error")
	}
	if !strings.Contains(err.Error(), "chunked_io") || !strings.Contains(err.Error(), "streaming") {
		t.Fatalf("GenerateFile() error = %q, want chunked_io streaming validation failure", err)
	}
}

func TestGenerateFileRejectsChunkedIOWithoutBytesChunkField(t *testing.T) {
	file := buildTestFile(t, []*descriptorpb.DescriptorProto{
		messageDescriptor("DownloadRequest", stringField("id", 1)),
		messageDescriptor("SnapshotChunk", stringField("data", 1)),
	}, []*descriptorpb.MethodDescriptorProto{
		methodDescriptor("Download", "DownloadRequest", "SnapshotChunk", false, true, &natspb.ChunkedIOOptions{
			ChunkField:       "data",
			DefaultChunkSize: 65536,
		}),
	})

	gen, target := newTestPlugin(t, file)
	lang := NewGoLanguage()

	err := GenerateFile(gen, target, lang)
	if err == nil {
		t.Fatal("GenerateFile() succeeded, want bytes-field validation error")
	}
	if !strings.Contains(err.Error(), "bytes") || !strings.Contains(err.Error(), "SnapshotChunk") {
		t.Fatalf("GenerateFile() error = %q, want bytes-field validation failure", err)
	}
}

func TestGenerateFileRejectsChunkedIOOnBidiMethod(t *testing.T) {
	file := buildTestFile(t, []*descriptorpb.DescriptorProto{
		messageDescriptor("SnapshotChunk", bytesField("data", 1)),
	}, []*descriptorpb.MethodDescriptorProto{
		methodDescriptor("Sync", "SnapshotChunk", "SnapshotChunk", true, true, &natspb.ChunkedIOOptions{
			ChunkField:       "data",
			DefaultChunkSize: 65536,
		}),
	})

	gen, target := newTestPlugin(t, file)
	lang := NewGoLanguage()

	err := GenerateFile(gen, target, lang)
	if err == nil {
		t.Fatal("GenerateFile() succeeded, want bidi chunked_io validation error")
	}
	if !strings.Contains(err.Error(), "bidirectional") {
		t.Fatalf("GenerateFile() error = %q, want bidirectional streaming validation failure", err)
	}
}

func TestGenerateFileRejectsChunkedIOWithExtraFields(t *testing.T) {
	file := buildTestFile(t, []*descriptorpb.DescriptorProto{
		messageDescriptor("DownloadRequest", stringField("id", 1)),
		messageDescriptor("SnapshotChunk",
			bytesField("data", 1),
			stringField("checksum", 2),
		),
	}, []*descriptorpb.MethodDescriptorProto{
		methodDescriptor("Download", "DownloadRequest", "SnapshotChunk", false, true, &natspb.ChunkedIOOptions{
			ChunkField:       "data",
			DefaultChunkSize: 65536,
		}),
	})

	gen, target := newTestPlugin(t, file)
	lang := NewGoLanguage()

	err := GenerateFile(gen, target, lang)
	if err == nil {
		t.Fatal("GenerateFile() succeeded, want simple chunk message validation error")
	}
	if !strings.Contains(err.Error(), "contain only bytes field") || !strings.Contains(err.Error(), "got 2 fields") {
		t.Fatalf("GenerateFile() error = %q, want simple chunk message validation failure with field count", err)
	}
}

func TestGenerateFileEmitsChunkedHelpersForValidStreamingMethods(t *testing.T) {
	file := buildTestFile(t, []*descriptorpb.DescriptorProto{
		messageDescriptor("DownloadRequest", stringField("id", 1)),
		messageDescriptor("UploadResponse", stringField("id", 1)),
		messageDescriptor("SnapshotChunk", bytesField("data", 1)),
	}, []*descriptorpb.MethodDescriptorProto{
		methodDescriptor("Download", "DownloadRequest", "SnapshotChunk", false, true, &natspb.ChunkedIOOptions{
			ChunkField:       "data",
			DefaultChunkSize: 65536,
		}),
		methodDescriptor("Upload", "SnapshotChunk", "UploadResponse", true, false, &natspb.ChunkedIOOptions{
			ChunkField:       "data",
			DefaultChunkSize: 65536,
		}),
	})

	gen, target := newTestPlugin(t, file)
	lang := NewGoLanguage()

	shared := gen.NewGeneratedFile("test/shared_nats.pb.go", target.GoImportPath)
	if err := lang.GenerateShared(shared, target); err != nil {
		t.Fatalf("GenerateShared() error = %v", err)
	}

	if err := GenerateFile(gen, target, lang); err != nil {
		t.Fatalf("GenerateFile() error = %v", err)
	}

	responseFiles := gen.Response().File
	if len(responseFiles) == 0 {
		t.Fatal("GenerateFile() produced no files")
	}

	// Collect generated file contents by suffix
	fileContents := make(map[string]string)
	for _, f := range responseFiles {
		fileContents[f.GetName()] = f.GetContent()
	}

	// Find the main service file (not shared, not chunked)
	var mainFile string
	for name, content := range fileContents {
		if strings.HasSuffix(name, "_nats.pb.go") &&
			!strings.HasSuffix(name, "shared_nats.pb.go") &&
			!strings.HasSuffix(name, "_chunked_nats.pb.go") &&
			!strings.HasSuffix(name, "_chunked_protoopaque_nats.pb.go") {
			mainFile = content
			break
		}
	}
	if mainFile == "" {
		t.Fatal("failed to find generated Go service file")
	}

	// Recv helpers should be in the main file
	recvSnippets := []string{
		"func (s *BlobService_Download_ClientStream) RecvBytes(ctx context.Context) ([]byte, error)",
		"func (s *BlobService_Download_ClientStream) RecvToWriter(ctx context.Context, w io.Writer) error",
		"func (s *BlobService_Download_ClientStream) RecvToFile(ctx context.Context, path string) error",
	}
	for _, snippet := range recvSnippets {
		if !strings.Contains(mainFile, snippet) {
			t.Errorf("main file missing recv snippet %q", snippet)
		}
	}

	// RecvToFile should use writeFileAtomically (atomic write)
	if !strings.Contains(mainFile, "writeFileAtomically") {
		t.Error("RecvToFile should use writeFileAtomically for atomic writes")
	}

	// Send helpers should NOT be in the main file
	sendSnippets := []string{"SendBytes", "SendReader", "SendFile"}
	for _, snippet := range sendSnippets {
		if strings.Contains(mainFile, "func (s *BlobService_Upload_ClientStream) "+snippet) {
			t.Errorf("main file should not contain send helper %q (moved to build-tagged files)", snippet)
		}
	}

	// Find the open-mode chunked send file
	var chunkedFile string
	for name, content := range fileContents {
		if strings.HasSuffix(name, "_chunked_nats.pb.go") {
			chunkedFile = content
			break
		}
	}
	if chunkedFile == "" {
		t.Fatal("failed to find chunked send file (*_chunked_nats.pb.go)")
	}

	// Verify build tag
	if !strings.Contains(chunkedFile, "!protoopaque") {
		t.Error("chunked send file should have //go:build !protoopaque")
	}

	// Send helpers should be in the chunked file
	openSendSnippets := []string{
		"func (s *BlobService_Upload_ClientStream) SendBytes(data []byte) error",
		"func (s *BlobService_Upload_ClientStream) SendReader(r io.Reader, chunkSize int) error",
		"func (s *BlobService_Upload_ClientStream) SendFile(path string, chunkSize int) error",
	}
	for _, snippet := range openSendSnippets {
		if !strings.Contains(chunkedFile, snippet) {
			t.Errorf("chunked file missing send snippet %q", snippet)
		}
	}

	// Open-mode SendBytes should use struct literal
	if !strings.Contains(chunkedFile, "SnapshotChunk{") {
		t.Error("open-mode SendBytes should use struct literal construction")
	}

	// Find the opaque-mode chunked send file
	var opaqueFile string
	for name, content := range fileContents {
		if strings.HasSuffix(name, "_chunked_protoopaque_nats.pb.go") {
			opaqueFile = content
			break
		}
	}
	if opaqueFile == "" {
		t.Fatal("failed to find opaque chunked send file (*_chunked_protoopaque_nats.pb.go)")
	}

	// Verify opaque build tag
	if !strings.Contains(opaqueFile, "protoopaque") || strings.Contains(opaqueFile, "!protoopaque") {
		t.Error("opaque chunked file should have //go:build protoopaque (without !)")
	}

	// Opaque-mode SendBytes should use setter
	if !strings.Contains(opaqueFile, ".SetData(data)") {
		t.Error("opaque-mode SendBytes should use .SetData() setter")
	}
}

func newTestPlugin(t *testing.T, file *descriptorpb.FileDescriptorProto) (*protogen.Plugin, *protogen.File) {
	t.Helper()

	return newTestPluginWithFiles(t, file.GetName(), file)
}

func newTestPluginWithFiles(t *testing.T, fileToGenerate string, files ...*descriptorpb.FileDescriptorProto) (*protogen.Plugin, *protogen.File) {
	t.Helper()

	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{fileToGenerate},
		ProtoFile:      files,
	}

	gen, err := protogen.Options{}.New(req)
	if err != nil {
		t.Fatalf("protogen.Options.New() error = %v", err)
	}

	target := gen.FilesByPath[fileToGenerate]
	if target == nil {
		t.Fatalf("failed to resolve generated file %q", fileToGenerate)
	}

	return gen, target
}

func buildTestFile(t *testing.T, messages []*descriptorpb.DescriptorProto, methods []*descriptorpb.MethodDescriptorProto) *descriptorpb.FileDescriptorProto {
	t.Helper()

	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test/blob.proto"),
		Package: proto.String("test.v1"),
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String("github.com/franchb/protoc-gen-nats-micro/test/blob;blobv1"),
		},
		MessageType: messages,
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name:   proto.String("BlobService"),
			Method: methods,
		}},
		Syntax: proto.String("proto3"),
	}
}

func methodDescriptor(name, input, output string, clientStreaming, serverStreaming bool, chunked *natspb.ChunkedIOOptions) *descriptorpb.MethodDescriptorProto {
	opts := &descriptorpb.MethodOptions{}
	if chunked != nil {
		proto.SetExtension(opts, natspb.E_ChunkedIo, chunked)
	}

	return &descriptorpb.MethodDescriptorProto{
		Name:            proto.String(name),
		InputType:       proto.String(".test.v1." + input),
		OutputType:      proto.String(".test.v1." + output),
		ClientStreaming: proto.Bool(clientStreaming),
		ServerStreaming: proto.Bool(serverStreaming),
		Options:         opts,
	}
}

func messageDescriptor(name string, fields ...*descriptorpb.FieldDescriptorProto) *descriptorpb.DescriptorProto {
	return &descriptorpb.DescriptorProto{
		Name:  proto.String(name),
		Field: fields,
	}
}

func bytesField(name string, number int32) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(number),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:   descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum(),
	}
}

func stringField(name string, number int32) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(number),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
	}
}
