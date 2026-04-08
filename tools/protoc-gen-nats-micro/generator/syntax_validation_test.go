package generator

import (
	"os/exec"
	"strings"
	"testing"

	natspb "github.com/franchb/protoc-gen-nats-micro/tools/protoc-gen-nats-micro/nats/micro"
	"google.golang.org/protobuf/types/descriptorpb"
)

// findPython3 locates a Python 3 interpreter on PATH.
// Tries "python3" first, then falls back to "python" if it reports version 3.x.
func findPython3(t *testing.T) (string, bool) {
	t.Helper()
	if bin, err := exec.LookPath("python3"); err == nil {
		return bin, true
	}
	bin, err := exec.LookPath("python")
	if err != nil {
		return "", false
	}
	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil || !strings.HasPrefix(string(out), "Python 3") {
		return "", false
	}
	return bin, true
}

// assertPythonSyntax validates that code is syntactically correct Python by
// running ast.parse() via the given interpreter. Code is passed through stdin
// to avoid shell-escaping issues with large generated output.
func assertPythonSyntax(t *testing.T, pythonBin, filename, code string) {
	t.Helper()
	cmd := exec.Command(pythonBin, "-c", "import ast,sys; ast.parse(sys.stdin.read())")
	cmd.Stdin = strings.NewReader(code)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("Python syntax error in generated file %q:\n%s", filename, string(out))
	}
}

// TestPythonGeneratedCodeSyntax generates Python code that exercises every
// template code path (unary, server-streaming, client-streaming, chunked
// download, chunked upload) and validates it as syntactically correct Python.
func TestPythonGeneratedCodeSyntax(t *testing.T) {
	pythonBin, found := findPython3(t)
	if !found {
		t.Skip("python3 not found on PATH; skipping Python syntax validation")
	}

	// Build a comprehensive proto that exercises every Python code path.
	file := buildTestFile(t, []*descriptorpb.DescriptorProto{
		messageDescriptor("GetRequest", stringField("id", 1)),
		messageDescriptor("GetResponse", stringField("value", 1)),
		messageDescriptor("ListRequest", stringField("filter", 1)),
		messageDescriptor("ListResponse", stringField("item", 1)),
		messageDescriptor("SumRequest", stringField("value", 1)),
		messageDescriptor("SumResponse", stringField("total", 1)),
		messageDescriptor("SnapshotChunk", bytesField("data", 1)),
		messageDescriptor("DownloadRequest", stringField("id", 1)),
		messageDescriptor("UploadResponse", stringField("id", 1)),
	}, []*descriptorpb.MethodDescriptorProto{
		// Unary
		methodDescriptor("Get", "GetRequest", "GetResponse", false, false, nil),
		// Server-streaming (plain)
		methodDescriptor("ListItems", "ListRequest", "ListResponse", false, true, nil),
		// Client-streaming (plain)
		methodDescriptor("Sum", "SumRequest", "SumResponse", true, false, nil),
		// Server-streaming + chunked_io
		methodDescriptor("Download", "DownloadRequest", "SnapshotChunk", false, true, &natspb.ChunkedIOOptions{
			ChunkField:       "data",
			DefaultChunkSize: 65536,
		}),
		// Client-streaming + chunked_io
		methodDescriptor("Upload", "SnapshotChunk", "UploadResponse", true, false, &natspb.ChunkedIOOptions{
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

	// Collect all generated Python files and validate syntax.
	responseFiles := gen.Response().File
	if len(responseFiles) == 0 {
		t.Fatal("no files were generated")
	}

	var found_py bool
	for _, f := range responseFiles {
		if strings.HasSuffix(f.GetName(), ".py") {
			found_py = true
			name := f.GetName()
			content := f.GetContent()
			t.Run(name, func(t *testing.T) {
				assertPythonSyntax(t, pythonBin, name, content)
			})
		}
	}
	if !found_py {
		t.Fatal("no Python files found in generated output")
	}
}
