package generator

// TypeScriptLanguage implements Language for TypeScript code generation
type TypeScriptLanguage struct{ BaseLanguage }

// SupportsJSON returns false — the TS templates only emit binary protobuf
// encode/decode paths, so the (nats.micro.service).json option is rejected
// at generation time rather than silently ignored.
func (t *TypeScriptLanguage) SupportsJSON() bool { return false }

// NewTypeScriptLanguage creates a new TypeScript language generator
func NewTypeScriptLanguage() *TypeScriptLanguage {
	return &TypeScriptLanguage{newBaseLanguage("typescript", "_nats.pb.ts", "templates/ts/*.tmpl",
		[]string{"header.ts.tmpl"},
		[]string{"shared_header.ts.tmpl", "shared.ts.tmpl"},
		[]string{"errors.ts.tmpl", "service.ts.tmpl", "client.ts.tmpl"},
	)}
}
