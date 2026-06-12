package generator

import (
	"bytes"
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
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

// GenerateExtraFiles emits two build-tagged files containing the chunked
// upload helpers (SendBytes, SendReader, SendFile) when any non-skipped
// client-streaming method has chunked_io enabled:
//   - *_chunked_nats.pb.go             — open-struct mode (//go:build !protoopaque)
//   - *_chunked_protoopaque_nats.pb.go — opaque mode      (//go:build protoopaque)
func (g *GoLanguage) GenerateExtraFiles(gen *protogen.Plugin, file *protogen.File, prefix string, importPath protogen.GoImportPath) error {
	if !hasClientStreamingChunkedIO(file) {
		return nil
	}
	entries := []struct{ suffix, template string }{
		{"_chunked_nats.pb.go", "chunked_send.go.tmpl"},
		{"_chunked_protoopaque_nats.pb.go", "chunked_send_opaque.go.tmpl"},
	}
	for _, e := range entries {
		out := gen.NewGeneratedFile(prefix+e.suffix, importPath)
		if err := g.executeFileTemplate(out, file, e.template); err != nil {
			return fmt.Errorf("execute %s: %w", e.template, err)
		}
	}
	return nil
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
