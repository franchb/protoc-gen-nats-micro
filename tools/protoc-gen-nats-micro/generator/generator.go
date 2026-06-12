package generator

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
)

// GenerateFile generates NATS microservice code for a protobuf file.
// The Language must be resolved by the caller (main.go).
func GenerateFile(gen *protogen.Plugin, file *protogen.File, lang Language) error {
	// Skip files with no generatable services entirely — emitting just the
	// header would produce a file full of unused imports.
	if !HasGeneratableServices(file) {
		return nil
	}

	importPath := ImportPathFor(file, lang)
	filenamePrefix := OutputFilenamePrefix(file, lang)
	filename := filenamePrefix + lang.FileExtension()
	g := gen.NewGeneratedFile(filename, importPath)

	// Generate header (package, imports)
	if err := lang.GenerateHeader(g, file); err != nil {
		return fmt.Errorf("generate header: %w", err)
	}

	// Generate each service
	for _, service := range file.Services {
		opts := GetServiceOptions(service)

		// Skip this service if skip is set to true
		if opts.Skip {
			continue
		}

		if err := ValidateServiceOptions(lang, opts); err != nil {
			return fmt.Errorf("service %s: %w", service.GoName, err)
		}

		for _, method := range service.Methods {
			methodOpts := GetEndpointOptions(method)
			if methodOpts.Skip {
				continue
			}
			if err := ValidateMethodOptions(method); err != nil {
				return fmt.Errorf("validate method %s: %w", method.GoName, err)
			}
		}

		if err := lang.Generate(g, file, service, opts); err != nil {
			return fmt.Errorf("generate service %s: %w", service.GoName, err)
		}
	}

	// Language-specific extra per-file outputs (e.g., Go chunked send helpers).
	if err := lang.GenerateExtraFiles(gen, file, filenamePrefix, importPath); err != nil {
		return fmt.Errorf("generate extra files: %w", err)
	}

	return nil
}

// HasGeneratableServices reports whether the file contains at least one
// service that is not marked skip.
func HasGeneratableServices(file *protogen.File) bool {
	for _, service := range file.Services {
		if !GetServiceOptions(service).Skip {
			return true
		}
	}
	return false
}

// OutputFilenamePrefix returns the per-file output path prefix (without
// extension): Go-like languages use the go_package-derived
// GeneratedFilenamePrefix; others mirror the proto source path
// (e.g., "auth/v1/auth.proto" -> "auth/v1/auth").
func OutputFilenamePrefix(file *protogen.File, lang Language) string {
	if lang.IsGoLike() {
		return file.GeneratedFilenamePrefix
	}
	return strings.TrimSuffix(file.Proto.GetName(), ".proto")
}

// ImportPathFor returns the Go import path for generated files; empty for
// non-Go languages, which resolve output paths from the proto source instead.
func ImportPathFor(file *protogen.File, lang Language) protogen.GoImportPath {
	if lang.IsGoLike() {
		return file.GoImportPath
	}
	return ""
}

// hasClientStreamingChunkedIO reports whether any method in file is a
// client-streaming method with chunked_io enabled.
func hasClientStreamingChunkedIO(file *protogen.File) bool {
	for _, svc := range file.Services {
		if GetServiceOptions(svc).Skip {
			continue
		}
		for _, m := range svc.Methods {
			opts := GetEndpointOptions(m)
			if !opts.Skip && opts.ChunkedIO != nil && IsClientStreaming(m) && !IsServerStreaming(m) {
				return true
			}
		}
	}
	return false
}

// ToSnakeCase converts CamelCase to snake_case, handling acronyms correctly.
// e.g., "HTTPServer" -> "http_server", "getHTTPSURL" -> "get_https_url"
func ToSnakeCase(s string) string {
	var result strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 {
			prev := runes[i-1]
			if r >= 'A' && r <= 'Z' {
				if prev >= 'a' && prev <= 'z' {
					// lowercase → uppercase: "getH" → "get_h"
					result.WriteByte('_')
				} else if prev >= '0' && prev <= '9' {
					// digit → uppercase: "V2O" → "v2_o"
					result.WriteByte('_')
				} else if prev >= 'A' && prev <= 'Z' && i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z' {
					// End of acronym before lowercase: "HTTPSe" → "http_se"
					result.WriteByte('_')
				}
			}
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// ToLowerFirst converts first character to lowercase
func ToLowerFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToLower(s[:1]) + s[1:]
}
