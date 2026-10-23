# First-party protos to edition 2026

**Date:** 2026-10-22
**Status:** Approved design, not yet implemented

## Goal

Move this repo's own protobuf schemas to edition 2026 with that edition's defaults, generate opaque Go, and stop vendoring `google/api`. The plugin must accept edition files without dropping proto2 or proto3 inputs.

## Decisions

| Decision | Choice |
| --- | --- |
| Edition | `2026` on every first-party proto. Not 2023 or 2024. |
| Field presence | Edition default: explicit. No `IMPLICIT` override. |
| Enums and repeated fields | Edition defaults: open enums, packed repeated fields. |
| Go API | `option features.(pb.go).api_level = API_OPAQUE` on every converted file. |
| Symbol visibility | Edition default: `STRICT`. `export` only the symbols listed below. |
| Naming style | Edition default: `STYLE2026`. Rename a field only if `buf build` rejects it. |
| Google APIs | Delete `third_party/googleapis`. Depend on `buf.build/googleapis/googleapis`. |
| Plugin window | `EDITION_PROTO2` through `EDITION_2026`. |
| Go protobuf module | `google.golang.org/protobuf` v1.37.0-rc.1, and `protoc-gen-go` from that same tag. |
| Buf | v1.75.0 or newer locally. CI installs v1.75.0. |
| Repo Go version | Stays `go 1.27.0`. The rc requires Go 1.26, which this repo already exceeds. |

## Non-goals

- Rewriting `REVIEW.md`.
- Changing TypeScript or Python generator APIs. Regenerate them only far enough to see whether they still compile.
- Dropping proto2 or proto3 support in the plugin.
- Pinning proto3 field presence.
- Tracking generated trees that are gitignored.
- Expanding CI into a language matrix.

## Schema

Fifteen files. `extensions/proto/natsmicro/options.proto`, plus:

- `examples/protos/common/location/v1/address.proto`
- `examples/protos/common/metadata/v1/metadata.proto`
- `examples/protos/common/types/v1/money.proto`
- `examples/protos/common/types/v1/status.proto`
- `examples/protos/common/types/v1/timestamp.proto`
- `examples/protos/demo/v1/encoding.proto`
- `examples/protos/example/v1/service.proto`
- `examples/protos/kvstore_demo/v1/service.proto`
- `examples/protos/order/v1/fulfillment.proto`
- `examples/protos/order/v1/service.proto`
- `examples/protos/order/v2/service.proto`
- `examples/protos/product/v1/service.proto`
- `examples/protos/streaming/v1/service.proto`
- `examples/protos/user/v1/service.proto`

Each file replaces `syntax = "proto3"` with:

```protobuf
edition = "2026";

import "google/protobuf/go_features.proto";

option features.(pb.go).api_level = API_OPAQUE;
```

No other feature options. `google/protobuf/go_features.proto` comes with protoc. It is not a Buf dependency.

Explicit presence means a scalar that is set to zero is serialized, and an unset field is omitted. Message fields, repeated fields, and maps keep their current cardinality. `options.proto` is the file users copy. After this change, regenerating it requires Buf v1.75.0 or newer and `protoc-gen-go` v1.37.0-rc.1, and the Go structs are opaque.

### `STRICT` exports

Visibility affects only cross-file proto references. It does not change generated Go, TypeScript, or Python. Services do not take `export` or `local`. Nested types cannot be exported. Do not lift types unless `buf build` shows a nested type referenced from another file. None are today.

Mark these top-level symbols `export`:

- `Money`, `Status`, `Timestamp`, `Metadata`, `Address`
- `KVWriteMode`, `KVPersistFailurePolicy`
- `ServiceOptions`, `EndpointOptions`, `KVStoreOptions`, `ObjectStoreOptions`, `StreamOptions`, `ChunkedIOOptions`

Request and response messages stay local. They are named only by the service in the same file.

`(google.api.http)` stays on the six example services that already use it (`demo`, `example`, `order/v1` service, `order/v1` fulfillment, `order/v2`, `product`). Those imports remain proto3 files from the Buf module. An edition 2026 file may import them.

`STYLE2026` forbids, inside one message, `has_x`, `set_x`, `get_x`, `clear_x`, and `x_value` next to a field `x`, and it forbids a field named `descriptor`. No current first-party field matches. If `buf build` reports one, rename it in this same change.

## Plugin

`tools/protoc-gen-nats-micro/main.go` today sets only `FEATURE_PROTO3_OPTIONAL`. Protoc rejects an edition file before any template runs. Inside the existing `Run` callback, set:

```go
gen.SupportedFeatures = uint64(
    pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL |
        pluginpb.CodeGeneratorResponse_FEATURE_SUPPORTS_EDITIONS,
)
gen.SupportedEditionsMinimum = descriptorpb.Edition_EDITION_PROTO2
gen.SupportedEditionsMaximum = descriptorpb.Edition_EDITION_2026
```

Proto2, proto3, edition 2023, edition 2024, and edition 2026 are accepted. A newer edition stays rejected until the maximum is raised on purpose.

The generator reads services, methods, names, and custom options through `protogen`. Service, client, and stream templates already call `Get*` or marshal whole messages. They do not change for the edition bump.

Chunked `SendBytes` is the one generated field write. It writes the RPC input message, so the Go API is that message's resolved level: the defining file's `features.(pb.go).api_level`, then a message-level override. If neither is set, edition 2024 and newer count as opaque, and older files count as open. A service file whose chunked inputs mix those APIs emits both shapes.

| Resolved API | Emitted chunked helper |
| --- | --- |
| Open (proto2, proto3, edition 2023 with no Go API option) | Today's tagged pair |
| Hybrid | Today's tagged pair |
| Opaque | One untagged file, setter body only. No composite-literal file. |

The setter body may be shared with the hybrid opaque template. The opaque-only output must not carry `//go:build protoopaque`. When every chunked input is opaque, that file is `*_chunked_nats.pb.go`. When the same file also has open or hybrid inputs, those keep the tagged pair and the opaque inputs are written to `*_chunked_opaque_nats.pb.go`.

## Buf dependency

Delete `third_party/googleapis`, including `README.vendor.md` and the vendored license. The root `buf.yaml` drops that module and its `MINIMAL` lint override. The workspace keeps `extensions/proto` and `examples/protos` and gains:

```yaml
deps:
  - buf.build/googleapis/googleapis
```

`buf dep update` writes `buf.lock` at the repo root. Commit the lock. Example generation runs from the repo root, so it resolves `google/api/annotations.proto` from that locked module.

`extensions/proto/buf.yaml` stays a separate module with no Buf dependency. `options.proto` imports only well-known types, and `task generate:extensions` runs with `dir: extensions/proto`.

Replace `TestBufWorkspaceUsesVendoredGoogleAPI` and `TestVendoredGoogleAPISubtreePresent` with file reads that require the root `buf.yaml` to depend on `buf.build/googleapis/googleapis`, to omit `third_party/googleapis`, and to have a `buf.lock` beside it. Those tests do not contact the registry.

## Generated Go

`protoc-gen-go` v1.37.0-rc.1 emits the opaque API for edition 2026. The explicit `api_level` option matches that default. Opaque messages expose `Get*`, `Set*`, `Has*`, and `Clear*`. Struct fields are unexported. Composite literals do not compile. Construct messages with builders: `pb.Foo_builder{Name: "x"}.Build()`. `Get*` on a scalar returns the zero value when the field is unset.

Hand-written Go that touches generated messages moves to getters and builders:

- `generator/options.go` and tests that construct `natspb` values
- `examples/complex-go`
- `examples/streaming-go`
- `examples/kvstore-go`

Option parsing keeps today's rule. A zero from `Get*` means "use the generator default". A present empty string does not become a new override. `GetTimeout() != nil` replaces `Timeout != nil`.

Regenerate and commit the tracked outputs:

- `gen/nats/micro/options.pb.go`
- `examples/streaming-go/gen/streaming/v1/service.pb.go`
- `examples/streaming-go/gen/streaming/v1/service_nats.pb.go`
- `examples/streaming-go/gen/streaming/v1/shared_nats.pb.go`

`examples/complex-go/gen`, `examples/simple-ts/gen`, and `examples/simple-py/gen` are gitignored. Regenerate them so the examples compile. Do not start tracking them.

TypeScript (`protoc-gen-ts`) and Python (builtin `python` / `pyi`) stay on their current plugins. `task generate:python` must succeed. Run `task generate:ts` and record the result. If `protoc-gen-ts` rejects edition 2026, leave the protos on 2026.

## Docs

Replace `syntax = "proto3"` with the edition 2026 header, including the Go feature import and `api_level = API_OPAQUE`, in:

- `README.md` usage sample
- `API.md` basic, production, and KV samples
- `docs/guide/quick-start.md`

Those samples are single files. Their messages stay local, with no `export`.

Replace the README "Vendored Proto Options" section. It tells the reader to copy `extensions/proto/natsmicro/options.proto`, to depend on `buf.build/googleapis/googleapis` when a service imports `google/api/annotations.proto`, and that generation needs Buf v1.75.0 or newer and `protoc-gen-go` v1.37.0-rc.1. The copied options file is opaque Go.

Leave `REVIEW.md`, `PYTHON.md`, `TYPESCRIPT.md`, and guides that do not show a syntax line unchanged. If a test pins the old snippet text, update that assertion in the same change.

## Testing

Required tools: Buf v1.75.0 or newer, and `protoc-gen-go` v1.37.0-rc.1.

- `buf lint` and `buf build` on the root workspace.
- `task generate:extensions`, then `go test` in `tools/protoc-gen-nats-micro`.
- A Go test runs `buf generate` through this plugin on a minimal edition 2026 fixture (one service, request, and response in a single file, with no googleapis import) and requires exit 0. The same test generates a proto3 fixture and requires exit 0. Building `descriptorpb` in memory does not satisfy this test. The test skips when `buf` is not on `PATH`.
- CI installs Buf v1.75.0 before `go test`, so that skip cannot happen on a pull request. The rest of CI stays the current lint job and the Go test job. No new language matrix.
- `go build ./...` at the repo root, and `go build` in `examples/complex-go`, `examples/streaming-go`, and `examples/kvstore-go`, after regeneration.
- An `API_OPAQUE` chunked client-streaming file emits one untagged setter helper and no composite-literal file. A proto3 file still emits the build-tagged pair.
- `task generate:python` succeeds. `task generate:ts` is run. Report success or the generator error in the implementation summary. Do not add a document for that result, and do not move the protos off edition 2026 if `protoc-gen-ts` rejects the edition.
- The replacement Buf-layout tests pass without a network fetch.

## Success criteria

- All fifteen first-party protos are `edition = "2026"` with `API_OPAQUE` and no presence override.
- `third_party/googleapis` is gone. `buf.lock` pins `buf.build/googleapis/googleapis`.
- The plugin advertises editions through 2026 and still generates a proto3 file.
- Tracked `*.pb.go` outputs are regenerated with `protoc-gen-go` v1.37.0-rc.1.
- The Go module builds, the plugin tests pass, and the three Go example modules build.
- Doc samples that used to say `syntax = "proto3"` say `edition = "2026"`.
