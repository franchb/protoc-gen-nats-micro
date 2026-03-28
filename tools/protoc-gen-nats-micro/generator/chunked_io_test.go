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
	if !strings.Contains(err.Error(), "contain only bytes field") {
		t.Fatalf("GenerateFile() error = %q, want simple chunk message validation failure", err)
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

	files := gen.Response().File
	if len(files) == 0 {
		t.Fatal("GenerateFile() produced no files")
	}

	var generated string
	for _, f := range files {
		if strings.HasSuffix(f.GetName(), "_nats.pb.go") && !strings.HasSuffix(f.GetName(), "shared_nats.pb.go") {
			generated = f.GetContent()
			break
		}
	}
	if generated == "" {
		t.Fatal("failed to find generated Go service file")
	}

	wantSnippets := []string{
		"func (s *BlobService_Download_ClientStream) RecvBytes(ctx context.Context) ([]byte, error)",
		"func (s *BlobService_Download_ClientStream) RecvToWriter(ctx context.Context, w io.Writer) error",
		"func (s *BlobService_Download_ClientStream) RecvToFile(ctx context.Context, path string) error",
		"func (s *BlobService_Upload_ClientStream) SendBytes(data []byte) error",
		"func (s *BlobService_Upload_ClientStream) SendReader(r io.Reader, chunkSize int) error",
		"func (s *BlobService_Upload_ClientStream) SendFile(path string, chunkSize int) error",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated Go output missing snippet %q\n%s", snippet, generated)
		}
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
