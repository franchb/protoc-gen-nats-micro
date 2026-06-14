package generator

import (
	"bytes"
	"embed"
	"fmt"
	"path"
	"strconv"
	"strings"
	"text/template"
	"time"

	"google.golang.org/protobuf/compiler/protogen"
)

//go:embed templates/*
var templatesFS embed.FS

// Language represents a target programming language for code generation
type Language interface {
	// Name returns the language name (e.g., "go", "typescript")
	Name() string

	// FileExtension returns the file extension (e.g., "_nats.pb.go", "_nats.pb.ts")
	FileExtension() string

	// IsGoLike returns whether this language uses Go import paths and
	// GeneratedFilenamePrefix for output path resolution. Non-Go languages
	// derive paths from the proto source file name instead.
	IsGoLike() bool

	// GenerateHeader generates the file header (package declaration, imports)
	GenerateHeader(g *protogen.GeneratedFile, file *protogen.File) error

	// GenerateShared generates shared code once per package (e.g., RegisterOption types, error codes)
	GenerateShared(g *protogen.GeneratedFile, file *protogen.File) error

	// Generate generates code for the given service
	Generate(g *protogen.GeneratedFile, file *protogen.File, service *protogen.Service, opts ServiceOptions) error

	// PostGenerate is called after shared file generation for any language-specific
	// post-processing (e.g., Python __init__.py). Default no-op in BaseLanguage.
	PostGenerate(gen *protogen.Plugin, file *protogen.File, pkgDir string) error

	// GenerateExtraFiles lets a language emit additional per-proto-file outputs
	// (e.g., Go's build-tagged chunked send helpers). Default no-op in BaseLanguage.
	GenerateExtraFiles(gen *protogen.Plugin, file *protogen.File, prefix string, importPath protogen.GoImportPath) error

	// SupportsJSON reports whether the language honors the
	// (nats.micro.service).json encoding option. Default true in BaseLanguage.
	SupportsJSON() bool
}

// TemplateData holds data passed to templates
type TemplateData struct {
	File          *protogen.File
	Service       *protogen.Service
	Options       ServiceOptions
	GeneratedFile *protogen.GeneratedFile
}

func (d TemplateData) QualifiedGoIdent(ident protogen.GoIdent) string {
	if d.GeneratedFile == nil {
		return ident.GoName
	}
	return d.GeneratedFile.QualifiedGoIdent(ident)
}

// BaseLanguage provides a reusable implementation of Language backed by Go templates.
// All language targets embed this struct and configure it with their template names.
type BaseLanguage struct {
	name             string
	extension        string
	templates        *template.Template
	headerTemplates  []string // Templates to execute for GenerateHeader
	sharedTemplates  []string // Templates to execute for GenerateShared
	serviceTemplates []string // Templates to execute for Generate (per-service)
}

// newBaseLanguage constructs a BaseLanguage with parsed templates from the embedded FS.
func newBaseLanguage(name, extension, glob string, headerTmpls, sharedTmpls, serviceTmpls []string) BaseLanguage {
	tmpl := template.Must(template.New(name).Funcs(FuncMap()).ParseFS(templatesFS, glob))
	return BaseLanguage{
		name:             name,
		extension:        extension,
		templates:        tmpl,
		headerTemplates:  headerTmpls,
		sharedTemplates:  sharedTmpls,
		serviceTemplates: serviceTmpls,
	}
}

func (b *BaseLanguage) Name() string          { return b.name }
func (b *BaseLanguage) FileExtension() string { return b.extension }
func (b *BaseLanguage) IsGoLike() bool        { return false }

func (b *BaseLanguage) PostGenerate(gen *protogen.Plugin, file *protogen.File, pkgDir string) error {
	return nil
}

func (b *BaseLanguage) GenerateExtraFiles(gen *protogen.Plugin, file *protogen.File, prefix string, importPath protogen.GoImportPath) error {
	return nil
}

func (b *BaseLanguage) SupportsJSON() bool { return true }

func (b *BaseLanguage) GenerateHeader(g *protogen.GeneratedFile, file *protogen.File) error {
	return b.executeTemplates(g, TemplateData{File: file, GeneratedFile: g}, b.headerTemplates)
}

func (b *BaseLanguage) GenerateShared(g *protogen.GeneratedFile, file *protogen.File) error {
	return b.executeTemplates(g, TemplateData{File: file, GeneratedFile: g}, b.sharedTemplates)
}

func (b *BaseLanguage) Generate(g *protogen.GeneratedFile, file *protogen.File, service *protogen.Service, opts ServiceOptions) error {
	return b.executeTemplates(g, TemplateData{File: file, Service: service, Options: opts, GeneratedFile: g}, b.serviceTemplates)
}

// executeTemplates runs each named template in order, writing output to g.
func (b *BaseLanguage) executeTemplates(g *protogen.GeneratedFile, data TemplateData, templateNames []string) error {
	for _, name := range templateNames {
		var buf bytes.Buffer
		if err := b.templates.ExecuteTemplate(&buf, name, data); err != nil {
			return fmt.Errorf("execute template %s: %w", name, err)
		}
		g.P(buf.String())
		g.P()
	}
	return nil
}

// FuncMap returns template helper functions
func FuncMap() template.FuncMap {
	return template.FuncMap{
		"ToSnakeCase":        ToSnakeCase,
		"ToLowerFirst":       ToLowerFirst,
		"ToCamelCase":        ToCamelCase,
		"ToPascalCase":       ToPascalCase,
		"GetServiceOptions":  GetServiceOptions,
		"GetEndpointOptions": GetEndpointOptions,
		"ProtoBasename":      ProtoBasename,
		// Duration rendering (always produces valid literals in the target language)
		"ToGoDuration": ToGoDuration,
		"ToMillis":     ToMillis,
		"ToPySeconds":  ToPySeconds,
		// Streaming detection
		"IsServerStreaming": IsServerStreaming,
		"IsClientStreaming": IsClientStreaming,
		"IsBidiStreaming":   IsBidiStreaming,
		"IsUnary":           IsUnary,
		// KV/ObjectStore key template resolution
		"ResolveKeyTemplateGo":       ResolveKeyTemplateGo,
		"ResolveKeyTemplateTS":       ResolveKeyTemplateTS,
		"ResolveKeyTemplatePy":       ResolveKeyTemplatePy,
		"IsKVWriteModeCompareAndSet": IsKVWriteModeCompareAndSet,
		"IsKVWriteModeCreateOnly":    IsKVWriteModeCreateOnly,
		"IsKVPersistFailureRequired": IsKVPersistFailureRequired,
	}
}

// ProtoBasename returns the base name of a proto file without extension
// e.g., "path/to/service.proto" -> "service"
func ProtoBasename(filename string) string {
	return strings.TrimSuffix(path.Base(filename), ".proto")
}

// ToGoDuration renders a duration as a Go source expression
// (e.g., 30s -> "30 * time.Second", 500ms -> "500 * time.Millisecond").
func ToGoDuration(d time.Duration) string {
	switch {
	case d == 0:
		return "0"
	case d%time.Second == 0:
		return fmt.Sprintf("%d * time.Second", d/time.Second)
	case d%time.Millisecond == 0:
		return fmt.Sprintf("%d * time.Millisecond", d/time.Millisecond)
	default:
		return fmt.Sprintf("time.Duration(%d)", int64(d))
	}
}

// ToMillis renders a duration as an integer millisecond literal for TS/JS.
// Positive sub-millisecond durations round up to 1ms so a configured timeout
// never collapses to 0 (which TS treats as "no timeout").
func ToMillis(d time.Duration) string {
	ms := d / time.Millisecond
	if d > 0 && d%time.Millisecond != 0 {
		ms++
	}
	return strconv.FormatInt(int64(ms), 10)
}

// ToPySeconds renders a duration as a valid Python float literal in seconds
// (e.g., 500ms -> "0.5", 30s -> "30.0").
func ToPySeconds(d time.Duration) string {
	s := strconv.FormatFloat(d.Seconds(), 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// ToUpperFirst converts first character to uppercase
func ToUpperFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ToCamelCase converts snake_case to CamelCase
func ToCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		parts[i] = ToUpperFirst(part)
	}
	return strings.Join(parts, "")
}

// ToPascalCase converts SCREAMING_SNAKE_CASE to PascalCase.
// e.g., "ORDER_EXPIRED" → "OrderExpired", "PAYMENT_FAILED" → "PaymentFailed"
func ToPascalCase(s string) string {
	return ToCamelCase(strings.ToLower(s))
}

// GetLanguage returns a language generator by name
func GetLanguage(name string) (Language, error) {
	switch strings.ToLower(name) {
	case "go", "golang":
		return NewGoLanguage(), nil
	case "typescript", "ts":
		return NewTypeScriptLanguage(), nil
	case "python", "py":
		return NewPythonLanguage(), nil
	case "web-ts", "webts":
		return NewWebTSLanguage(), nil
	default:
		return nil, fmt.Errorf("unsupported language: %s", name)
	}
}
