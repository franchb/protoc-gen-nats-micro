package generator

// WebTSLanguage implements Language for web-focused TypeScript code generation.
// It generates client-only code compatible with protoc-gen-es v2 (@bufbuild/protobuf).
type WebTSLanguage struct{ BaseLanguage }

// SupportsJSON returns false — see TypeScriptLanguage.SupportsJSON.
func (w *WebTSLanguage) SupportsJSON() bool { return false }

// NewWebTSLanguage creates a new Web TypeScript language generator
func NewWebTSLanguage() *WebTSLanguage {
	return &WebTSLanguage{newBaseLanguage("web-ts", "_nats.pb.ts", "templates/web-ts/*.tmpl",
		[]string{"header.web-ts.tmpl"},
		[]string{"shared_header.web-ts.tmpl", "shared.web-ts.tmpl"},
		[]string{"errors.web-ts.tmpl", "client.web-ts.tmpl"},
	)}
}
