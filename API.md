# API Reference

Complete reference for `protoc-gen-nats-micro` proto extension options.

## Table of Contents

- [Service Options](#service-options)
- [Endpoint Options](#endpoint-options)
- [Stream Options](#stream-options)
- [Chunked I/O](#chunked-io)
- [Generated Code Reference](#generated-code-reference)
- [Runtime Configuration Priority](#runtime-configuration-priority)

## Service Options

Service-level configuration applied to the entire service with `option (natsmicro.service)`.

### Supported Fields

| Field | Type | Description |
| --- | --- | --- |
| `subject_prefix` | `string` | NATS subject prefix for all endpoints |
| `name` | `string` | Service name used for registration |
| `version` | `string` | Semantic version string |
| `description` | `string` | Human-readable service description |
| `metadata` | `map<string,string>` | Service discovery metadata |
| `timeout` | `google.protobuf.Duration` | Default timeout for all endpoints |
| `skip` | `bool` | Skip NATS generation for the service |
| `json` | `bool` | Use JSON instead of binary protobuf |
| `error_codes` | `repeated string` | Additional generated error codes |
| `queue_group_disabled` | `bool` | Register grouped endpoints without a queue group |

```protobuf
service ProductService {
  option (natsmicro.service) = {
    subject_prefix: "api.v1"
    name: "product_service"
    version: "2.0.0"
    description: "Product catalog management"
    timeout: {seconds: 30}
    metadata: {key: "team" value: "catalog"}
    queue_group_disabled: true
    error_codes: ["OUT_OF_STOCK", "PRICE_CHANGED"]
  };
}
```

## Endpoint Options

Method-level configuration with `option (natsmicro.endpoint)`.

| Field | Type | Description |
| --- | --- | --- |
| `timeout` | `google.protobuf.Duration` | Override the service default timeout |
| `skip` | `bool` | Skip NATS generation for this method |
| `metadata` | `map<string,string>` | Endpoint-specific metadata |
| `queue_group_disabled` | `bool` | Use a plain subscription for this endpoint |
| `pending_msg_limit` | `int32` | NATS pending message limit; `-1` disables the limit |
| `pending_bytes_limit` | `int32` | NATS pending byte limit; `-1` disables the limit |

```protobuf
rpc SearchProducts(SearchRequest) returns (SearchResponse) {
  option (natsmicro.endpoint) = {
    timeout: {seconds: 60}
    metadata: {key: "operation" value: "read"}
    pending_msg_limit: 1024
    pending_bytes_limit: 1048576
  };
}
```

## Stream Options

Streaming flow-control hints with `option (natsmicro.stream)`.

| Field | Type | Description |
| --- | --- | --- |
| `max_inflight` | `int32` | Max concurrent in-flight messages |
| `ordered` | `bool` | Emit sequence headers for ordered delivery |

## Chunked I/O

Chunked I/O is the fork-specific large-payload path. It generates Go helpers that bridge streaming RPCs to `io.Reader` and `io.Writer`, which you can then wire to NATS ObjectStore in your application code.

### Proto Annotation

```protobuf
message SnapshotChunk {
  bytes data = 1;
}

rpc ExportSnapshot(ExportSnapshotRequest) returns (stream SnapshotChunk) {
  option (natsmicro.chunked_io) = {};
}

rpc ImportSnapshot(stream SnapshotChunk) returns (ImportSnapshotResponse) {
  option (natsmicro.chunked_io) = {
    default_chunk_size: 131072
  };
}
```

### Supported Fields

| Field | Type | Description |
| --- | --- | --- |
| `chunk_field` | `string` | Bytes field name inside the chunk message |
| `default_chunk_size` | `int32` | Default helper chunk size when callers pass `0` |

### Constraints

- Go-only in the current release.
- Supported on server-streaming downloads and client-streaming uploads.
- Not supported on bidirectional methods.
- The chunk message must contain exactly one `bytes` field that matches `chunk_field`.

### Generated Helpers

For download streams:

- `RecvBytes(ctx)`
- `RecvToWriter(ctx, w)`
- `RecvToFile(ctx, path)`

For upload streams:

- `SendBytes(data)`
- `SendReader(r, chunkSize)`
- `SendFile(path, chunkSize)`

## Generated Code Reference

### Go Service Registration

```go
func RegisterProductServiceHandlers(
    nc *nats.Conn,
    impl ProductServiceNats,
    opts ...RegisterOption,
) (ProductServiceWrapper, error)

func WithSubjectPrefix(prefix string) RegisterOption
func WithName(name string) RegisterOption
func WithVersion(version string) RegisterOption
func WithDescription(desc string) RegisterOption
func WithTimeout(timeout time.Duration) RegisterOption
func WithMetadata(metadata map[string]string) RegisterOption
func WithAdditionalMetadata(metadata map[string]string) RegisterOption
func WithServerInterceptor(interceptor UnaryServerInterceptor) RegisterOption
```

### Go Client Creation

```go
func NewProductServiceNatsClient(
    nc *nats.Conn,
    opts ...NatsClientOption,
) *ProductServiceNatsClient

func WithNatsClientSubjectPrefix(prefix string) NatsClientOption
func WithClientInterceptor(interceptor UnaryClientInterceptor) NatsClientOption
```

### Error Handling

Generated Go packages include:

- built-in error code constants such as `INVALID_ARGUMENT`, `NOT_FOUND`, and `INTERNAL`
- constructors like `NewProductServiceInvalidArgumentError`
- predicate helpers like `IsProductServiceNotFound`

### Headers

Generated helpers expose request and response header accessors on both client and server paths.

### Streaming Helpers

Generated Go stream wrappers expose typed `Send`, `Recv`, `Close`, `CloseSend`, and `CloseAndRecv` methods depending on the RPC shape, plus chunked I/O helpers when `chunked_io` is enabled.

## Runtime Configuration Priority

Options apply in this order:

1. Runtime overrides such as `WithTimeout()`
2. Endpoint-level proto options
3. Service-level proto options
4. Built-in defaults

## See Also

- [README.md](README.md)
- [docs/guide/streaming.md](docs/guide/streaming.md)
- [extensions/proto/natsmicro/options.proto](extensions/proto/natsmicro/options.proto)
- [examples/complex-go/](examples/complex-go/)
- [examples/streaming-go/](examples/streaming-go/)
