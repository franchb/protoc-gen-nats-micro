package generator

import (
	"bytes"
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/gofeaturespb"
)

// GoLanguage implements Language for Go code generation
type GoLanguage struct{ BaseLanguage }

// IsGoLike returns true — Go uses Go import paths and GeneratedFilenamePrefix.
func (g *GoLanguage) IsGoLike() bool { return true }

// NewGoLanguage creates a new Go language generator
func NewGoLanguage() *GoLanguage {
	return &GoLanguage{newBaseLanguage("go", "_nats.pb.go", "templates/go/*.tmpl",
		[]string{"header.go.tmpl"},
		[]string{"shared_header.go.tmpl", "shared.go.tmpl", "stream_helpers.go.tmpl"},
		[]string{"errors.go.tmpl", "service.go.tmpl", "stream.go.tmpl", "client.go.tmpl"},
	)}
}

// GenerateExtraFiles emits chunked upload helpers when any non-skipped
// client-streaming method has chunked_io enabled. SendBytes writes the RPC
// input message, so the Go API is that message's resolved APILevel: the
// defining file's default, then a message-level features.(pb.go).api_level.
//
// API_OPAQUE inputs get one untagged setter file. When every chunked input
// in this file is opaque, that file is *_chunked_nats.pb.go. Open and hybrid
// inputs keep the build-tagged pair. A file that mixes both also emits
// *_chunked_opaque_nats.pb.go for the opaque inputs, because
// *_chunked_nats.pb.go is already the !protoopaque file.
func (g *GoLanguage) GenerateExtraFiles(gen *protogen.Plugin, file *protogen.File, prefix string, importPath protogen.GoImportPath) error {
	if !hasClientStreamingChunkedIO(file) {
		return nil
	}
	hasOpaque, hasOpen := chunkedUploadKinds(file)
	if hasOpaque && !hasOpen {
		return g.emitChunkedFile(gen, file, prefix+"_chunked_nats.pb.go", importPath, "chunked_send_opaque_default.go.tmpl")
	}
	if hasOpen {
		if err := g.emitChunkedFile(gen, file, prefix+"_chunked_nats.pb.go", importPath, "chunked_send.go.tmpl"); err != nil {
			return err
		}
		if err := g.emitChunkedFile(gen, file, prefix+"_chunked_protoopaque_nats.pb.go", importPath, "chunked_send_opaque.go.tmpl"); err != nil {
			return err
		}
	}
	if hasOpaque && hasOpen {
		return g.emitChunkedFile(gen, file, prefix+"_chunked_opaque_nats.pb.go", importPath, "chunked_send_opaque_default.go.tmpl")
	}
	return nil
}

func (g *GoLanguage) emitChunkedFile(gen *protogen.Plugin, file *protogen.File, filename string, importPath protogen.GoImportPath, tmpl string) error {
	out := gen.NewGeneratedFile(filename, importPath)
	if err := g.executeFileTemplate(out, file, tmpl); err != nil {
		return fmt.Errorf("execute %s: %w", tmpl, err)
	}
	return nil
}

// inputIsOpaque reports whether method's input message is pure API_OPAQUE.
// Open and hybrid messages keep the build-tagged SendBytes pair.
func inputIsOpaque(method *protogen.Method) bool {
	return method != nil && method.Input != nil &&
		method.Input.APILevel == gofeaturespb.GoFeatures_API_OPAQUE
}

func chunkedUploadKinds(file *protogen.File) (opaque, open bool) {
	for _, svc := range file.Services {
		if GetServiceOptions(svc).Skip {
			continue
		}
		for _, m := range svc.Methods {
			opts := GetEndpointOptions(m)
			if opts.Skip || opts.ChunkedIO == nil || !IsClientStreaming(m) || IsServerStreaming(m) {
				continue
			}
			if inputIsOpaque(m) {
				opaque = true
			} else {
				open = true
			}
		}
	}
	return opaque, open
}

// executeFileTemplate executes a single named template with file-level data
// (no per-service context) and writes the result to g.
func (g *GoLanguage) executeFileTemplate(out *protogen.GeneratedFile, file *protogen.File, tmplName string) error {
	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, tmplName, TemplateData{File: file, GeneratedFile: out}); err != nil {
		return fmt.Errorf("execute template %s: %w", tmplName, err)
	}
	out.P(buf.String())
	return nil
}
