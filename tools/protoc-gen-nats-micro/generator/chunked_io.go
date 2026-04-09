package generator

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	defaultChunkFieldName = "data"
	defaultChunkSize      = 64 * 1024
)

// ValidateMethodOptions validates additive generator features that depend on
// method shape before templates run.
func ValidateMethodOptions(method *protogen.Method) error {
	opts := GetEndpointOptions(method)
	if opts.ChunkedIO != nil {
		if err := ValidateChunkedIO(method, opts.ChunkedIO); err != nil {
			return err
		}
	}
	return nil
}

// ValidateChunkedIO ensures chunked I/O helpers are only enabled for supported
// streaming shapes with a simple single-bytes chunk message.
func ValidateChunkedIO(method *protogen.Method, opts *ChunkedIOOpts) error {
	if opts == nil {
		return nil
	}

	if IsUnary(method) {
		return fmt.Errorf("method %s: chunked_io is only valid on streaming methods", method.GoName)
	}
	if IsBidiStreaming(method) {
		return fmt.Errorf("method %s: chunked_io is not supported on bidirectional streaming methods", method.GoName)
	}
	if opts.DefaultChunkSize < 0 {
		return fmt.Errorf("method %s: chunked_io default_chunk_size must be >= 0", method.GoName)
	}

	msg := chunkedIOMessage(method)
	if msg == nil {
		return fmt.Errorf("method %s: chunked_io could not resolve streamed message", method.GoName)
	}
	if len(msg.Fields) != 1 {
		return fmt.Errorf(
			"method %s: chunked_io requires streamed message %s to contain only bytes field %q (got %d fields)",
			method.GoName,
			msg.GoIdent.GoName,
			opts.ChunkField,
			len(msg.Fields),
		)
	}

	field := msg.Fields[0]
	if string(field.Desc.Name()) != opts.ChunkField {
		return fmt.Errorf(
			"method %s: chunked_io requires streamed message %s to contain bytes field %q",
			method.GoName,
			msg.GoIdent.GoName,
			opts.ChunkField,
		)
	}
	if field.Desc.Cardinality() == protoreflect.Repeated {
		return fmt.Errorf(
			"method %s: chunked_io field %q on %s must be singular bytes, not repeated",
			method.GoName,
			opts.ChunkField,
			msg.GoIdent.GoName,
		)
	}
	if field.Desc.Kind() != protoreflect.BytesKind {
		return fmt.Errorf(
			"method %s: chunked_io field %q on %s must be bytes",
			method.GoName,
			opts.ChunkField,
			msg.GoIdent.GoName,
		)
	}

	return nil
}

func chunkedIOMessage(method *protogen.Method) *protogen.Message {
	switch {
	case IsServerStreaming(method) && !IsClientStreaming(method):
		return method.Output
	case IsClientStreaming(method) && !IsServerStreaming(method):
		return method.Input
	default:
		return nil
	}
}
