package generator

import (
	"bytes"
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
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
// client-streaming method has chunked_io enabled.
// Pure API_OPAQUE files, and editions files that default to opaque, get one
// untagged file:
//   - *_chunked_nats.pb.go — opaque setter (no build tag)
//
// Open and hybrid files keep the build-tagged pair:
//   - *_chunked_nats.pb.go             — open-struct mode (//go:build !protoopaque)
//   - *_chunked_protoopaque_nats.pb.go — opaque mode      (//go:build protoopaque)
func (g *GoLanguage) GenerateExtraFiles(gen *protogen.Plugin, file *protogen.File, prefix string, importPath protogen.GoImportPath) error {
	if !hasClientStreamingChunkedIO(file) {
		return nil
	}
	if useOpaqueChunkedHelper(file) {
		out := gen.NewGeneratedFile(prefix+"_chunked_nats.pb.go", importPath)
		if err := g.executeFileTemplate(out, file, "chunked_send_opaque_default.go.tmpl"); err != nil {
			return fmt.Errorf("execute chunked_send_opaque_default.go.tmpl: %w", err)
		}
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

// useOpaqueChunkedHelper reports whether this file's Go messages are pure
// API_OPAQUE. Hybrid and open files keep the build-tagged pair. An editions
// file with no explicit api_level is opaque from edition 2024 up.
func useOpaqueChunkedHelper(file *protogen.File) bool {
	features := file.Proto.GetOptions().GetFeatures()
	if features != nil && proto.HasExtension(features, gofeaturespb.E_Go) {
		level := proto.GetExtension(features, gofeaturespb.E_Go).(*gofeaturespb.GoFeatures).GetApiLevel()
		switch level {
		case gofeaturespb.GoFeatures_API_OPAQUE:
			return true
		case gofeaturespb.GoFeatures_API_OPEN, gofeaturespb.GoFeatures_API_HYBRID:
			return false
		}
	}
	return file.Proto.GetSyntax() == "editions" &&
		file.Proto.GetEdition() >= descriptorpb.Edition_EDITION_2024
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
