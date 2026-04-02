package generator

import (
	"strings"
	"testing"

	natspb "github.com/toyz/protoc-gen-nats-micro/tools/protoc-gen-nats-micro/nats/micro"
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

	// SendReader and SendFile should be in the main file (not build-tagged)
	mainSendSnippets := []string{
		"func (s *BlobService_Upload_ClientStream) SendReader(r io.Reader, chunkSize int) error",
		"func (s *BlobService_Upload_ClientStream) SendFile(path string, chunkSize int) error",
	}
	for _, snippet := range mainSendSnippets {
		if !strings.Contains(mainFile, snippet) {
			t.Errorf("main file missing send snippet %q", snippet)
		}
	}

	// SendBytes should NOT be in the main file (lives in build-tagged files)
	if strings.Contains(mainFile, "func (s *BlobService_Upload_ClientStream) SendBytes") {
		t.Error("main file should not contain SendBytes (moved to build-tagged files)")
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

	// Only SendBytes should be in the chunked file
	if !strings.Contains(chunkedFile, "func (s *BlobService_Upload_ClientStream) SendBytes(data []byte) error") {
		t.Error("chunked file missing SendBytes")
	}

	// SendReader and SendFile should NOT be in the chunked file (moved to main)
	if strings.Contains(chunkedFile, "SendReader") {
		t.Error("chunked file should not contain SendReader (moved to main file)")
	}
	if strings.Contains(chunkedFile, "SendFile") {
		t.Error("chunked file should not contain SendFile (moved to main file)")
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

func TestGenerateFileEmitsChunkedHelpersForTypeScript(t *testing.T) {
	// Only server-streaming (download) — TS has no client-streaming support yet.
	file := buildTestFile(t, []*descriptorpb.DescriptorProto{
		messageDescriptor("DownloadRequest", stringField("id", 1)),
		messageDescriptor("SnapshotChunk", bytesField("data", 1)),
	}, []*descriptorpb.MethodDescriptorProto{
		methodDescriptor("Download", "DownloadRequest", "SnapshotChunk", false, true, &natspb.ChunkedIOOptions{
			ChunkField:       "data",
			DefaultChunkSize: 65536,
		}),
	})

	gen, target := newTestPlugin(t, file)
	lang := NewTypeScriptLanguage()

	shared := gen.NewGeneratedFile("test/shared_nats.pb.ts", "")
	if err := lang.GenerateShared(shared, target); err != nil {
		t.Fatalf("GenerateShared() error = %v", err)
	}

	if err := GenerateFile(gen, target, lang); err != nil {
		t.Fatalf("GenerateFile() error = %v", err)
	}

	responseFiles := gen.Response().File
	var tsFile string
	for _, f := range responseFiles {
		if strings.HasSuffix(f.GetName(), "_nats.pb.ts") && !strings.HasPrefix(f.GetName(), "test/shared") {
			tsFile = f.GetContent()
			break
		}
	}
	if tsFile == "" {
		t.Fatal("failed to find generated TypeScript service file")
	}

	// Chunked receiver subclass should be generated
	if !strings.Contains(tsFile, "class BlobService_Download_ChunkedReceiver extends ClientStreamReceiver") {
		t.Error("TS file missing ChunkedReceiver subclass")
	}

	// recvBytes method should be present
	if !strings.Contains(tsFile, "async recvBytes(): Promise<Uint8Array>") {
		t.Error("TS file missing recvBytes() method signature")
	}

	// Field access should use camelCase (data → data)
	if !strings.Contains(tsFile, "msg.data") {
		t.Error("TS file should access chunk field as msg.data")
	}

	// Method should return chunked receiver type
	if !strings.Contains(tsFile, "Promise<BlobService_Download_ChunkedReceiver>") {
		t.Error("TS file should return ChunkedReceiver from download method")
	}

	// Constructor should use chunked receiver
	if !strings.Contains(tsFile, "new BlobService_Download_ChunkedReceiver(sub") {
		t.Error("TS file should construct ChunkedReceiver instance")
	}
}

func TestGenerateFileEmitsClientStreamingForTypeScript(t *testing.T) {
	file := buildTestFile(t, []*descriptorpb.DescriptorProto{
		messageDescriptor("SumRequest", stringField("value", 1)),
		messageDescriptor("SumResponse", stringField("total", 1)),
	}, []*descriptorpb.MethodDescriptorProto{
		methodDescriptor("Sum", "SumRequest", "SumResponse", true, false, nil),
	})

	gen, target := newTestPlugin(t, file)
	lang := NewTypeScriptLanguage()

	shared := gen.NewGeneratedFile("test/shared_nats.pb.ts", "")
	if err := lang.GenerateShared(shared, target); err != nil {
		t.Fatalf("GenerateShared() error = %v", err)
	}

	if err := GenerateFile(gen, target, lang); err != nil {
		t.Fatalf("GenerateFile() error = %v", err)
	}

	responseFiles := gen.Response().File
	var tsFile string
	for _, f := range responseFiles {
		if strings.HasSuffix(f.GetName(), "_nats.pb.ts") && !strings.HasPrefix(f.GetName(), "test/shared") {
			tsFile = f.GetContent()
			break
		}
	}
	if tsFile == "" {
		t.Fatal("failed to find generated TypeScript service file")
	}

	// ClientStreamSender class should be generated
	if !strings.Contains(tsFile, "ClientStreamSender") {
		t.Error("TS file missing ClientStreamSender class")
	}

	// closeAndRecv method should be present
	if !strings.Contains(tsFile, "closeAndRecv") {
		t.Error("TS file missing closeAndRecv() method")
	}

	// send method should be present
	if !strings.Contains(tsFile, "send(") {
		t.Error("TS file missing send() method")
	}

	// Nats-Stream-Inbox header should be used in the service handler
	if !strings.Contains(tsFile, "Nats-Stream-Inbox") {
		t.Error("TS file missing Nats-Stream-Inbox header usage")
	}

	// Client method should initiate handshake
	if !strings.Contains(tsFile, "Nats-Stream-Inbox") {
		t.Error("TS file should use Nats-Stream-Inbox in handshake")
	}

	// Interface should have client-streaming method
	if !strings.Contains(tsFile, "sum(): Promise<ClientStreamSender<") {
		t.Error("TS interface missing client-streaming method signature")
	}

	// Service interface should have client-streaming handler
	if !strings.Contains(tsFile, "sum(stream: AsyncIterableIterator<") {
		t.Error("TS service interface missing client-streaming handler signature")
	}
}

func TestGenerateFileEmitsChunkedUploadForTypeScript(t *testing.T) {
	file := buildTestFile(t, []*descriptorpb.DescriptorProto{
		messageDescriptor("SnapshotChunk", bytesField("data", 1)),
		messageDescriptor("UploadResponse", stringField("id", 1)),
	}, []*descriptorpb.MethodDescriptorProto{
		methodDescriptor("Upload", "SnapshotChunk", "UploadResponse", true, false, &natspb.ChunkedIOOptions{
			ChunkField:       "data",
			DefaultChunkSize: 65536,
		}),
	})

	gen, target := newTestPlugin(t, file)
	lang := NewTypeScriptLanguage()

	shared := gen.NewGeneratedFile("test/shared_nats.pb.ts", "")
	if err := lang.GenerateShared(shared, target); err != nil {
		t.Fatalf("GenerateShared() error = %v", err)
	}

	if err := GenerateFile(gen, target, lang); err != nil {
		t.Fatalf("GenerateFile() error = %v", err)
	}

	responseFiles := gen.Response().File
	var tsFile string
	for _, f := range responseFiles {
		if strings.HasSuffix(f.GetName(), "_nats.pb.ts") && !strings.HasPrefix(f.GetName(), "test/shared") {
			tsFile = f.GetContent()
			break
		}
	}
	if tsFile == "" {
		t.Fatal("failed to find generated TypeScript service file")
	}

	// ChunkedClientStreamSender subclass should be generated
	if !strings.Contains(tsFile, "ChunkedClientStreamSender") {
		t.Error("TS file missing ChunkedClientStreamSender subclass")
	}

	// sendBytes method should be present
	if !strings.Contains(tsFile, "sendBytes") {
		t.Error("TS file missing sendBytes() method")
	}

	// ChunkedClientStreamSender should extend ClientStreamSender
	if !strings.Contains(tsFile, "extends ClientStreamSender") {
		t.Error("ChunkedClientStreamSender should extend ClientStreamSender")
	}

	// Method return type should be the chunked sender
	if !strings.Contains(tsFile, "Promise<BlobService_Upload_ChunkedClientStreamSender>") {
		t.Error("TS file should return ChunkedClientStreamSender from upload method")
	}

	// Nats-Stream-Inbox should be used
	if !strings.Contains(tsFile, "Nats-Stream-Inbox") {
		t.Error("TS file missing Nats-Stream-Inbox header usage")
	}
}

func TestGenerateFileEmitsChunkedHelpersForPython(t *testing.T) {
	// Only server-streaming (download) — Python has no client-streaming support yet.
	file := buildTestFile(t, []*descriptorpb.DescriptorProto{
		messageDescriptor("DownloadRequest", stringField("id", 1)),
		messageDescriptor("SnapshotChunk", bytesField("data", 1)),
	}, []*descriptorpb.MethodDescriptorProto{
		methodDescriptor("Download", "DownloadRequest", "SnapshotChunk", false, true, &natspb.ChunkedIOOptions{
			ChunkField:       "data",
			DefaultChunkSize: 65536,
		}),
	})

	gen, target := newTestPlugin(t, file)
	lang := NewPythonLanguage()

	shared := gen.NewGeneratedFile("test/shared_nats_pb2.py", "")
	if err := lang.GenerateShared(shared, target); err != nil {
		t.Fatalf("GenerateShared() error = %v", err)
	}

	if err := GenerateFile(gen, target, lang); err != nil {
		t.Fatalf("GenerateFile() error = %v", err)
	}

	responseFiles := gen.Response().File
	var pyFile string
	for _, f := range responseFiles {
		if strings.HasSuffix(f.GetName(), "_nats_pb2.py") && !strings.HasPrefix(f.GetName(), "test/shared") {
			pyFile = f.GetContent()
			break
		}
	}
	if pyFile == "" {
		t.Fatal("failed to find generated Python service file")
	}

	// Chunked receiver subclass should be generated
	if !strings.Contains(pyFile, "class BlobService_Download_ChunkedReceiver(ClientStreamReceiver)") {
		t.Error("Python file missing ChunkedReceiver subclass")
	}

	// recv_bytes method should be present
	if !strings.Contains(pyFile, "async def recv_bytes(self) -> bytes") {
		t.Error("Python file missing recv_bytes() method signature")
	}

	// Field access should use proto field name (data)
	if !strings.Contains(pyFile, "msg.data") {
		t.Error("Python file should access chunk field as msg.data")
	}

	// Method should return chunked receiver type
	if !strings.Contains(pyFile, "BlobService_Download_ChunkedReceiver") {
		t.Error("Python file should reference ChunkedReceiver")
	}
}

func TestValidateChunkedIOWorksForAllLanguages(t *testing.T) {
	// Validation is language-agnostic (runs before templates), but verify it
	// works when called through GenerateFile with each language.
	languages := []struct {
		name string
		lang Language
	}{
		{"Go", NewGoLanguage()},
		{"TypeScript", NewTypeScriptLanguage()},
		{"Python", NewPythonLanguage()},
	}

	// Bidi + chunked_io should be rejected for all languages
	file := buildTestFile(t, []*descriptorpb.DescriptorProto{
		messageDescriptor("SnapshotChunk", bytesField("data", 1)),
	}, []*descriptorpb.MethodDescriptorProto{
		methodDescriptor("Sync", "SnapshotChunk", "SnapshotChunk", true, true, &natspb.ChunkedIOOptions{
			ChunkField:       "data",
			DefaultChunkSize: 65536,
		}),
	})

	for _, tt := range languages {
		t.Run(tt.name, func(t *testing.T) {
			gen, target := newTestPlugin(t, file)
			err := GenerateFile(gen, target, tt.lang)
			if err == nil {
				t.Fatalf("%s: GenerateFile() succeeded, want bidi chunked_io validation error", tt.name)
			}
			if !strings.Contains(err.Error(), "bidirectional") {
				t.Fatalf("%s: GenerateFile() error = %q, want bidirectional validation failure", tt.name, err)
			}
		})
	}
}

func newTestPlugin(t *testing.T, file *descriptorpb.FileDescriptorProto) (*protogen.Plugin, *protogen.File) {
	t.Helper()

	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{file.GetName()},
		ProtoFile:      []*descriptorpb.FileDescriptorProto{file},
	}

	gen, err := protogen.Options{}.New(req)
	if err != nil {
		t.Fatalf("protogen.Options.New() error = %v", err)
	}

	target := gen.FilesByPath[file.GetName()]
	if target == nil {
		t.Fatalf("failed to resolve generated file %q", file.GetName())
	}

	return gen, target
}

func buildTestFile(t *testing.T, messages []*descriptorpb.DescriptorProto, methods []*descriptorpb.MethodDescriptorProto) *descriptorpb.FileDescriptorProto {
	t.Helper()

	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test/blob.proto"),
		Package: proto.String("test.v1"),
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String("github.com/toyz/protoc-gen-nats-micro/test/blob;blobv1"),
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
