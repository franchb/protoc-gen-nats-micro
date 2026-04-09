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
