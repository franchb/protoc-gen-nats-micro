# API Reference

Complete reference for `protoc-gen-nats-micro` proto extension options.

## Service Options

Service-level configuration using `option (natsmicro.service)`.

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `subject_prefix` | `string` | Snake-case of service name | NATS subject prefix for all endpoints |
| `name` | `string` | Service name | Service name for NATS micro registration |
| `version` | `string` | `"1.0.0"` | Service version |
| `description` | `string` | — | Human-readable description |
| `metadata` | `map<string, string>` | — | Service metadata for discovery and operations |
| `timeout` | `Duration` | No timeout | Default timeout for all endpoints |
| `skip` | `bool` | `false` | Skip NATS generation for this service |
| `json` | `bool` | `false` | Use JSON encoding instead of binary protobuf |
| `error_codes` | `repeated string` | — | Custom application-specific error codes |
| `queue_group_disabled` | `bool` | `false` | Register grouped endpoints as plain subscriptions instead of queue subscriptions |

```protobuf
service ProductService {
  option (natsmicro.service) = {
    subject_prefix: "api.v1"
    name: "product_service"
    version: "2.0.0"
    description: "Product catalog API"
    timeout: {seconds: 30}
    metadata: { key: "team" value: "catalog" }
    error_codes: ["OUT_OF_STOCK", "PRICE_CHANGED"]
  };
}
```

## Endpoint Options

Per-method configuration using `option (natsmicro.endpoint)`.

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `timeout` | `Duration` | Service timeout | Override timeout for this method |
| `skip` | `bool` | `false` | Skip NATS generation for this method |
| `metadata` | `map<string, string>` | — | Endpoint metadata |
| `queue_group_disabled` | `bool` | `false` | Register this endpoint as a plain subscription |
| `pending_msg_limit` | `int32` | `0` | Subscription pending message limit; `-1` disables the limit |
| `pending_bytes_limit` | `int32` | `0` | Subscription pending byte limit; `-1` disables the limit |

```protobuf
rpc CreateProduct(CreateReq) returns (CreateResp) {
  option (natsmicro.endpoint) = {
    timeout: {seconds: 10}
    metadata: { key: "operation" value: "write" }
    queue_group_disabled: true
    pending_msg_limit: 1024
    pending_bytes_limit: 1048576
  };
}
```

## Stream Options

Per-method streaming controls using `option (natsmicro.stream)`.

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `max_inflight` | `int32` | `0` | Max in-flight messages for generated streaming flow control |
| `ordered` | `bool` | `false` | Attach sequence headers for ordered delivery |

## Chunked I/O Options

Per-method helper generation for simple blob transfer over streaming RPC using `option (natsmicro.chunked_io)`.

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `chunk_field` | `string` | `"data"` | Name of the bytes field carrying each streamed chunk |
| `default_chunk_size` | `int32` | `65536` | Default helper chunk size in bytes |

Constraints:

- Go-only in the current release.
- Valid only on server-streaming and client-streaming methods.
- The streamed message must contain exactly one `bytes` field matching `chunk_field`.
- Metadata belongs in the unary request or final unary response, not in chunk messages.

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

Generated Go helpers:

- Download streams: `RecvBytes(ctx)`, `RecvToWriter(ctx, w)`, `RecvToFile(ctx, path)`
- Upload streams: `SendBytes(data)`, `SendReader(r, chunkSize)`, `SendFile(path, chunkSize)`

Use these helpers to connect your generated client to NATS ObjectStore in application code. The generator does not emit KV/ObjectStore persistence wrappers.

## Runtime Options

### Server Registration Options

| Option | Description |
| --- | --- |
| `WithName(name)` | Override service name |
| `WithVersion(version)` | Override version |
| `WithDescription(desc)` | Override description |
| `WithSubjectPrefix(prefix)` | Override subject prefix |
| `WithTimeout(duration)` | Override default timeout |
| `WithMetadata(map)` | Replace service metadata |
| `WithAdditionalMetadata(map)` | Merge into service metadata |
| `WithServerInterceptor(fn)` | Add server-side interceptor |
| `WithStatsHandler(fn)` | Set stats handler |
| `WithDoneHandler(fn)` | Set done handler |
| `WithErrorHandler(fn)` | Set error handler |

### Client Options

| Option | Description |
| --- | --- |
| `WithClientSubjectPrefix(prefix)` | Override subject prefix |
| `WithClientInterceptor(fn)` | Add client-side interceptor |

## Timeout Precedence

From highest to lowest priority:

1. Runtime override, such as `WithTimeout(60 * time.Second)`
2. Endpoint-level `option (natsmicro.endpoint)`
3. Service-level `option (natsmicro.service)`
4. No timeout

## Proto Import

Vendor the options proto into your repo, for example at `protos/natsmicro/options.proto`, and keep this import:

```protobuf
import "natsmicro/options.proto";
```

All options are defined in [extensions/proto/natsmicro/options.proto](https://github.com/franchb/protoc-gen-nats-micro/blob/main/extensions/proto/natsmicro/options.proto).
