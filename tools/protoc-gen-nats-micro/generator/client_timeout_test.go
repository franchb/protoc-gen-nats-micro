package generator

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"
)

func TestGenerateFileEmitsClientTimeoutForUnaryCalls(t *testing.T) {
	file := buildTestFile(t, []*descriptorpb.DescriptorProto{
		messageDescriptor("GetRequest", stringField("id", 1)),
		messageDescriptor("GetResponse", stringField("value", 1)),
		messageDescriptor("ListRequest", stringField("filter", 1)),
		messageDescriptor("ListResponse", stringField("item", 1)),
	}, []*descriptorpb.MethodDescriptorProto{
		methodDescriptor("GetItem", "GetRequest", "GetResponse", false, false, nil),
		methodDescriptor("ListItems", "ListRequest", "ListResponse", false, true, nil),
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

	var sharedFile, mainFile string
	for _, f := range gen.Response().File {
		switch {
		case strings.HasSuffix(f.GetName(), "shared_nats.pb.go"):
			sharedFile = f.GetContent()
		case strings.HasSuffix(f.GetName(), "_nats.pb.go") &&
			!strings.HasSuffix(f.GetName(), "_chunked_nats.pb.go") &&
			!strings.HasSuffix(f.GetName(), "_chunked_protoopaque_nats.pb.go"):
			mainFile = f.GetContent()
		}
	}
	if sharedFile == "" || mainFile == "" {
		t.Fatal("failed to find generated Go files")
	}

	for _, snippet := range []string{
		"func WithClientTimeout(timeout time.Duration) NatsClientOption",
		"func WithClientTimeoutFunc(fn ClientTimeoutFunc) NatsClientOption",
		"func clientCallContext(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc)",
		"time.Until(deadline) <= d",
		"return context.WithTimeout(ctx, d)",
	} {
		if !strings.Contains(sharedFile, snippet) {
			t.Fatalf("shared file missing snippet %q", snippet)
		}
	}

	for _, snippet := range []string{
		"timeout:       cfg.timeout,",
		"timeoutFunc:   cfg.timeoutFunc,",
		"func (c *BlobServiceNatsClient) timeoutFor(method string) time.Duration",
		"invokerCtx, cancel := clientCallContext(invokerCtx, c.timeoutFor(method))",
	} {
		if !strings.Contains(mainFile, snippet) {
			t.Fatalf("service file missing snippet %q", snippet)
		}
	}

	// The deadline wraps the unary NATS call only. ListItems is server-streaming.
	getIdx := strings.Index(mainFile, "func (c *BlobServiceNatsClient) GetItem(")
	listIdx := strings.Index(mainFile, "func (c *BlobServiceNatsClient) ListItems(")
	if getIdx < 0 || listIdx < 0 || listIdx < getIdx {
		t.Fatalf("GetItem at %d, ListItems at %d", getIdx, listIdx)
	}
	unary := mainFile[getIdx:listIdx]
	if !strings.Contains(unary, "clientCallContext") {
		t.Fatal("unary GetItem does not apply clientCallContext")
	}
	if strings.Contains(mainFile[listIdx:], "clientCallContext") {
		t.Fatal("server-streaming ListItems applies clientCallContext")
	}
}
