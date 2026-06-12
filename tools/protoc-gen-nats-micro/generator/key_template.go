package generator

import (
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
)

var keyTemplatePlaceholderRe = regexp.MustCompile(`\{(\w+)\}`)

// keyTemplateLiteralRe limits literal (non-placeholder) template text to the
// NATS KV key charset. Anything else (%, quotes, backticks, ${, stray braces)
// would corrupt the generated Go/TS/Python key expressions.
var keyTemplateLiteralRe = regexp.MustCompile(`^[A-Za-z0-9\-/_=.]*$`)

// ValidateKeyTemplate checks that the template only contains characters safe
// to splice into generated code, and that every {field} placeholder refers to
// an actual field on the method's input message. Returns an error with a
// clear message listing available fields if a placeholder is invalid.
func ValidateKeyTemplate(template string, method *protogen.Method) error {
	literal := keyTemplatePlaceholderRe.ReplaceAllString(template, "")
	if !keyTemplateLiteralRe.MatchString(literal) {
		return fmt.Errorf(
			"key_template %q contains unsupported characters: only [A-Za-z0-9-/_=.] and {field} placeholders are allowed",
			template,
		)
	}

	matches := keyTemplatePlaceholderRe.FindAllStringSubmatch(template, -1)
	if len(matches) == 0 {
		return nil // No placeholders, nothing else to validate
	}

	// Build a set of valid field names from the input message
	validFields := make(map[string]bool)
	var fieldNames []string
	for _, f := range method.Input.Fields {
		name := string(f.Desc.Name())
		validFields[name] = true
		fieldNames = append(fieldNames, name)
	}

	// Check each placeholder
	for _, m := range matches {
		fieldName := m[1]
		if !validFields[fieldName] {
			return fmt.Errorf(
				"key_template %q references field {%s} which does not exist on input message %s (available fields: [%s])",
				template,
				fieldName,
				method.Input.GoIdent.GoName,
				strings.Join(fieldNames, ", "),
			)
		}
	}
	return nil
}

// ResolveKeyTemplateGo converts a key template like "user.{id}" into Go code:
// fmt.Sprintf("user.%v", msg.GetId())
// Panics at code-gen time if a placeholder references a nonexistent field.
func ResolveKeyTemplateGo(template string, method *protogen.Method) string {
	if err := ValidateKeyTemplate(template, method); err != nil {
		panic(fmt.Sprintf("protoc-gen-nats-micro: %v", err))
	}

	matches := keyTemplatePlaceholderRe.FindAllStringSubmatch(template, -1)
	if len(matches) == 0 {
		return fmt.Sprintf("%q", template)
	}

	format := keyTemplatePlaceholderRe.ReplaceAllString(template, "%v")
	var args []string
	for _, m := range matches {
		fieldName := m[1]
		args = append(args, fmt.Sprintf("msg.Get%s()", goFieldNameFor(method, fieldName)))
	}

	return fmt.Sprintf("fmt.Sprintf(%q, %s)", format, strings.Join(args, ", "))
}

// ResolveKeyTemplateTS converts a key template like "user.{id}" into TypeScript code:
// `user.${req.id}`
// Panics at code-gen time if a placeholder references a nonexistent field.
func ResolveKeyTemplateTS(template string, method *protogen.Method) string {
	if err := ValidateKeyTemplate(template, method); err != nil {
		panic(fmt.Sprintf("protoc-gen-nats-micro: %v", err))
	}

	result := keyTemplatePlaceholderRe.ReplaceAllStringFunc(template, func(match string) string {
		fieldName := match[1 : len(match)-1] // strip { }
		tsFieldName := fieldNameToTSAccessor(fieldName)
		return fmt.Sprintf("${request.%s}", tsFieldName)
	})
	return fmt.Sprintf("`%s`", result)
}

// ResolveKeyTemplatePy converts a key template like "user.{id}" into Python code:
// f"user.{request_msg.id}"
// Panics at code-gen time if a placeholder references a nonexistent field.
func ResolveKeyTemplatePy(template string, method *protogen.Method) string {
	if err := ValidateKeyTemplate(template, method); err != nil {
		panic(fmt.Sprintf("protoc-gen-nats-micro: %v", err))
	}

	result := keyTemplatePlaceholderRe.ReplaceAllStringFunc(template, func(match string) string {
		fieldName := match[1 : len(match)-1] // strip { }
		return fmt.Sprintf("{request_msg.%s}", fieldName)
	})
	return fmt.Sprintf("f\"%s\"", result)
}

// goFieldNameFor resolves the Go field name from the protobuf descriptor so
// generated getters match protoc-gen-go's GoCamelCase exactly (e.g., "id_2"
// -> GetId_2). Falls back to naive conversion for names not present on the
// input message (ValidateKeyTemplate normally rules that out).
func goFieldNameFor(method *protogen.Method, fieldName string) string {
	for _, f := range method.Input.Fields {
		if string(f.Desc.Name()) == fieldName {
			return f.GoName
		}
	}
	return ToCamelCase(fieldName)
}

// fieldNameToTSAccessor converts a proto field name to a TypeScript accessor
// Proto uses snake_case, TS/JS generated code uses camelCase
// e.g., "user_id" -> "userId", "id" -> "id"
func fieldNameToTSAccessor(name string) string {
	parts := strings.Split(name, "_")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}
