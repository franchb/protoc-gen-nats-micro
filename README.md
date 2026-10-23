# protoc-gen-nats-micro

[![Go Version](https://img.shields.io/github/go-mod/go-version/franchb/protoc-gen-nats-micro)](https://github.com/franchb/protoc-gen-nats-micro)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/franchb/protoc-gen-nats-micro)](https://github.com/franchb/protoc-gen-nats-micro/releases/tag/v0.3.0)

> Fork of [Toyz/protoc-gen-nats-micro](https://github.com/Toyz/protoc-gen-nats-micro). Experimental, and usually a step ahead of upstream. PRs from this repo are backported there.

A Protocol Buffers compiler plugin that generates type-safe [NATS microservices](https://github.com/nats-io/nats.go/tree/main/micro) from `.proto` files.

**v0.3.0** is the Go release for Protobuf edition 2026 and the opaque Go API.

## Go, edition 2026, and the opaque API

The plugin accepts schemas from proto2 through edition 2026 (`EDITION_PROTO2` … `EDITION_2026`). Proto2 and proto3 inputs still generate. The schemas in this repository are edition 2026.

Every first-party file starts like this:

```protobuf
edition = "2026";

import "google/protobuf/go_features.proto";

option features.(pb.go).api_level = API_OPAQUE;
```

`API_OPAQUE` is the Go API those files generate. `protoc-gen-go` v1.37.0-rc.1 hides struct fields and emits builders and accessors:

- Build a message with `Order_builder{...}.Build()`.
- Singular scalars and explicit-presence enums are pointers on the builder: `proto.String`, `proto.Int32`, `proto.Int64`, `proto.Bool`, and `Status_PENDING.Enum()`.
- Repeated fields, maps, and nested messages are passed through.
- Read a field with `GetCustomerId()`. Write one on an existing message with `SetCustomerId(...)`.
- A nil builder field is unset and is omitted on the wire. A set zero is serialized. That is edition explicit presence. These files do not set `features.field_presence`.

Edition 2026 also defaults to `STRICT` symbol visibility and `STYLE2026` names. `export` is only on types that another file references (`Money`, `Status`, the NATS option messages, and the other shared types). Request and response messages stay local to their file.

Generate Go with:

| Tool | Version |
| --- | --- |
| Go | 1.27.0 or later |
| Buf | 1.75.0 or later (edition 2026 and well-known types at Protobuf 36.2) |
| `protoc-gen-go` | v1.37.0-rc.1 or a later release that supports edition 2026. v1.36 stops at edition 2024. |
| `google.golang.org/protobuf` | v1.37.0-rc.1, the module this repo requires |

`google/api/annotations.proto` stays proto3. An edition 2026 file may import it. Depend on the Buf module instead of vendoring it:

```yaml
version: v2
deps:
  - buf.build/googleapis/googleapis
```

Well-known types (`google/protobuf/go_features.proto`, `duration.proto`, `descriptor.proto`) come with protoc. They are not a Buf dependency.

### Other languages

The plugin also has TypeScript, browser TypeScript (`web-ts`), and Python templates. Edition 2026 support depends on the protobuf compiler beside this plugin:

- **Go** is the supported path for these schemas. `task generate:go` is the command to run.
- **Python** generation uses protoc's builtin `python` and `pyi` plugins. It accepts edition 2026 when `protoc` on `PATH` is 36.2 or newer. Older protoc stops with `Unrecognized syntax: editions`.
- **TypeScript** (`protoc-gen-ts` from `@protobuf-ts/plugin`) does not advertise edition support. `task generate:ts` does not succeed against the edition 2026 schemas in this repo. The protos stay on edition 2026.

## What the generator emits

From a service proto, `protoc-gen-nats-micro` writes:

- a server interface and `Register*Handlers`
- a typed NATS client
- streaming helpers when the RPC is streaming
- package-level shared types (`shared_nats.pb.go`): errors, register options, client options
- optional JetStream KV and Object Store persistence
- chunked upload and download helpers when `chunked_io` is set

`protoc-gen-go` writes the messages (`service.pb.go`). For an `API_OPAQUE` file, chunked Go uploads call `Set*` in one file with no build tag. Open and hybrid files keep the `protoopaque` build-tag pair.

The example Buf config also emits gRPC, a REST gateway, and OpenAPI so the demo can show those side by side. A NATS service does not need them.

## Features

- Service and endpoint metadata, timeouts, and subject prefixes live in the proto
- Typed errors, including custom `error_codes` from the proto
- Client and server interceptors, with request and response headers
- Server-streaming, client-streaming, and bidirectional streaming
- Chunked `io.Reader` / `io.Writer` helpers for large payloads
- JetStream KV and Object Store persistence, with write modes, TTL, and compression
- Per-endpoint pending limits and optional queue-group disable
- Several API versions on one connection, isolated by subject prefix
- One shared Go file per proto package

## Quick start

### Prerequisites

- Go 1.27.0 or later
- [Buf](https://buf.build/docs/installation) 1.75.0 or later
- `protoc-gen-go` v1.37.0-rc.1
- NATS server (Docker or local)
- [Task](https://taskfile.dev) if you want the repo commands

### Install

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.37.0-rc.1
go install github.com/franchb/protoc-gen-nats-micro/tools/protoc-gen-nats-micro@v0.3.0
```

`@latest` follows the default branch. Pin `@v0.3.0` when you want this release.

### Proto options

Copy [`extensions/proto/natsmicro/options.proto`](extensions/proto/natsmicro/options.proto) to `protos/natsmicro/options.proto` in your project. Service files keep `import "natsmicro/options.proto";`.

That file is edition 2026 and sets `API_OPAQUE`. Go code generated from it is opaque. Regenerating it needs the Buf and `protoc-gen-go` versions above.

### Generate and run the Go example

```bash
# from the repo root
task generate:go

# terminal 1
docker run --rm -p 4222:4222 nats

# terminal 2
go run examples/complex-go/server.go

# terminal 3
go run examples/complex-go/client.go
```

`task run:go:server` and `task run:go:client` are the same two programs. `examples/complex-go` has two `main` packages in one directory, so build each file on its own.

## Usage

### 1. Define the service

```protobuf
edition = "2026";

package order.v1;

import "google/protobuf/go_features.proto";
import "natsmicro/options.proto";
import "google/api/annotations.proto";
import "google/protobuf/duration.proto";

option features.(pb.go).api_level = API_OPAQUE;

service OrderService {
  option (natsmicro.service) = {
    subject_prefix: "api.v1"
    name: "order_service"
    version: "1.0.0"
    description: "Order management service"
    timeout: {seconds: 30}
    metadata: {
      key: "team"
      value: "orders"
    }
  };

  rpc CreateOrder(CreateOrderRequest) returns (CreateOrderResponse) {
    option (natsmicro.endpoint) = {
      metadata: {
        key: "operation"
        value: "write"
      }
    };
    option (google.api.http) = {
      post: "/v1/orders"
      body: "*"
    };
  }

  rpc GetOrder(GetOrderRequest) returns (GetOrderResponse) {
    option (natsmicro.endpoint) = {
      metadata: {
        key: "operation"
        value: "read"
      }
    };
    option (google.api.http) = {
      get: "/v1/orders/{id}"
    };
  }
}

message CreateOrderRequest {
  string customer_id = 1;
  repeated OrderItem items = 2;
}

message CreateOrderResponse {
  Order order = 1;
}

// Order, OrderItem, and GetOrder* live in the same file.
// They are local. Add `export` only if another file names them.
```

Request and response messages are not exported. Services do not take `export` or `local`. The full option reference is [API.md](API.md).

### 2. Implement the interface

Opaque messages are built and read as follows. `google.golang.org/protobuf/proto` supplies `proto.String` and the other pointer helpers.

```go
func (s *orderService) CreateOrder(
    ctx context.Context,
    req *orderv1.CreateOrderRequest,
) (*orderv1.CreateOrderResponse, error) {
    order := orderv1.Order_builder{
        Id:         proto.String(generateID()),
        CustomerId: proto.String(req.GetCustomerId()),
        Items:      req.GetItems(),
        Status:     orderv1.OrderStatus_PENDING.Enum(),
    }.Build()
    s.orders[order.GetId()] = order
    return orderv1.CreateOrderResponse_builder{Order: order}.Build(), nil
}

func (s *orderService) GetOrder(
    ctx context.Context,
    req *orderv1.GetOrderRequest,
) (*orderv1.GetOrderResponse, error) {
    order, ok := s.orders[req.GetId()]
    if !ok {
        return nil, errors.New("order not found")
    }
    return orderv1.GetOrderResponse_builder{Order: order}.Build(), nil
}
```

### 3. Register with NATS

```go
nc, err := nats.Connect(nats.DefaultURL)
if err != nil {
    log.Fatal(err)
}
defer nc.Close()

svc := &orderService{orders: make(map[string]*orderv1.Order)}

// Proto timeout is 30s. The service is registered at api.v1.order_service.
_, err = orderv1.RegisterOrderServiceHandlers(nc, svc)

// Runtime timeout replaces the proto timeout for this process.
_, err = orderv1.RegisterOrderServiceHandlers(nc, svc,
    orderv1.WithTimeout(45*time.Second),
)
```

### 4. Call it

```go
client := orderv1.NewOrderServiceNatsClient(nc)

resp, err := client.CreateOrder(context.Background(),
    orderv1.CreateOrderRequest_builder{
        CustomerId: proto.String("user-123"),
        Items: []*orderv1.OrderItem{
            orderv1.OrderItem_builder{
                ProductId: proto.String("prod-456"),
                Quantity:  proto.Int32(2),
            }.Build(),
        },
    }.Build(),
)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Created order: %s\n", resp.GetOrder().GetId())
```

## Generated files

```
gen/order/v1/
├── service.pb.go        # messages, protoc-gen-go, opaque API
├── service_nats.pb.go   # NATS server and client
└── shared_nats.pb.go    # errors and options for the whole package
```

Two services in package `order.v1` share one `shared_nats.pb.go`:

```
gen/order/v1/
├── service_nats.pb.go
├── fulfillment_nats.pb.go
└── shared_nats.pb.go
```

```go
type OrderServiceNats interface {
    CreateOrder(context.Context, *CreateOrderRequest) (*CreateOrderResponse, error)
    GetOrder(context.Context, *GetOrderRequest) (*GetOrderResponse, error)
}

func RegisterOrderServiceHandlers(nc *nats.Conn, impl OrderServiceNats, opts ...RegisterOption) (OrderServiceService, error)

func NewOrderServiceNatsClient(nc *nats.Conn, opts ...NatsClientOption) *OrderServiceNatsClient
```

The returned service embeds `micro.Service` (`Stop`, `Info`, `Stats`) and lists endpoints:

```go
svc, _ := productv1.RegisterProductServiceHandlers(nc, impl)
for _, ep := range svc.Endpoints() {
    fmt.Printf("%s -> %s\n", ep.Name, ep.Subject)
}
```

`ep.Name` and `ep.Subject` are NATS endpoint info, not protobuf fields.

## Configuration

Options are the `natsmicro` extensions. Snippets below assume the edition 2026 header from [Usage](#1-define-the-service) and `import "natsmicro/options.proto"`.

### Timeouts

Runtime wins over the endpoint, which wins over the service.

```protobuf
service ProductService {
  option (natsmicro.service) = {timeout: {seconds: 30}};

  rpc SearchProducts(SearchRequest) returns (SearchResponse) {
    option (natsmicro.endpoint) = {timeout: {seconds: 60}};
  }
}
```

```go
productv1.RegisterProductServiceHandlers(nc, svc,
    productv1.WithTimeout(45*time.Second),
)
```

`WithTimeout` bounds the handler. It does not stop a client whose context has no deadline. Unary clients take their own deadline:

```go
client := orderv1.NewOrderServiceNatsClient(nc,
    orderv1.WithClientTimeout(30*time.Second),
)

// A positive result overrides WithClientTimeout for that RPC.
// The name is the bare method, such as "GetOrder".
client = orderv1.NewOrderServiceNatsClient(nc,
    orderv1.WithClientTimeoutFunc(func(method string) time.Duration {
        if strings.HasPrefix(method, "Get") || strings.HasPrefix(method, "List") {
            return 45 * time.Second
        }
        return 90 * time.Second
    }),
)
```

An earlier deadline already on the context is kept. The deadline is fixed once per unary call and applied inside each NATS invoker call, after client interceptors. A retry does not reset it. Streaming calls are unchanged.

### Runtime overrides

```go
orderv1.RegisterOrderServiceHandlers(nc, svc,
    orderv1.WithSubjectPrefix("custom.prefix"),
    orderv1.WithVersion("2.0.0"),
    orderv1.WithTimeout(45*time.Second),
)
```

### Metadata

Service metadata is for the deployment. Endpoint metadata describes the RPC.

```protobuf
service ProductService {
  option (natsmicro.service) = {
    metadata: {key: "team" value: "platform"}
  };

  rpc GetProduct(GetProductRequest) returns (GetProductResponse) {
    option (natsmicro.endpoint) = {
      metadata: {key: "operation" value: "read"}
      metadata: {key: "cacheable" value: "true"}
    };
  }
}
```

```go
// Replace proto metadata.
productv1.RegisterProductServiceHandlers(nc, svc,
    productv1.WithMetadata(map[string]string{"custom": "value"}),
)

// Merge. This is the one to use for instance identity.
productv1.RegisterProductServiceHandlers(nc, svc,
    productv1.WithAdditionalMetadata(map[string]string{
        "instance_id": uuid.New().String(),
    }),
)
```

### Queue groups and pending limits

Queue groups are how NATS load-balances a service. Disable them when every subscriber should see the message. Pending limits are slow-consumer protection and map to `micro.WithEndpointPendingLimits`.

```protobuf
service ProductService {
  option (natsmicro.service) = {
    queue_group_disabled: true
  };

  rpc ImportCatalog(ImportCatalogRequest) returns (ImportCatalogResponse) {
    option (natsmicro.endpoint) = {
      pending_msg_limit: 128
      pending_bytes_limit: 1048576
    };
  }
}
```

An endpoint `queue_group_disabled` applies only to that RPC. A service flag applies to the grouped endpoints of that generated service.

### Skip

```protobuf
service AdminService {
  option (natsmicro.service) = {skip: true};
}

rpc AdminReset(ResetRequest) returns (ResetResponse) {
  option (natsmicro.endpoint) = {skip: true};
}
```

## API versioning

Subject prefixes keep two generated versions on one connection:

```go
orderv1.RegisterOrderServiceHandlers(nc, svcV1) // api.v1.order_service.*
orderv2.RegisterOrderServiceHandlers(nc, svcV2) // api.v2.order_service.*

clientV1 := orderv1.NewOrderServiceNatsClient(nc)
clientV2 := orderv2.NewOrderServiceNatsClient(nc)
```

## Interceptors and headers

### Server

Interceptors run in registration order, then the handler.

```go
func loggingInterceptor(ctx context.Context, req interface{}, info *productv1.UnaryServerInfo, handler productv1.UnaryHandler) (interface{}, error) {
    if headers := productv1.IncomingHeaders(ctx); headers != nil {
        if traceID, ok := headers["X-Trace-Id"]; ok && len(traceID) > 0 {
            log.Printf("[%s] Trace-ID: %s", info.Method, traceID[0])
        }
    }

    responseHeaders := nats.Header{}
    responseHeaders.Set("X-Server-Version", "1.0.0")
    productv1.SetResponseHeaders(ctx, responseHeaders)

    return handler(ctx, req)
}

productv1.RegisterProductServiceHandlers(nc, impl,
    productv1.WithServerInterceptor(authInterceptor),
    productv1.WithServerInterceptor(loggingInterceptor),
)
```

### Client

```go
func clientLoggingInterceptor(ctx context.Context, method string, req, reply interface{}, invoker productv1.UnaryInvoker) error {
    headers := nats.Header{}
    headers.Set("X-Trace-Id", generateTraceID())
    ctx = productv1.WithOutgoingHeaders(ctx, headers)

    err := invoker(ctx, method, req, reply)

    if respHeaders := productv1.ResponseHeaders(ctx); respHeaders != nil {
        if serverVer, ok := respHeaders["X-Server-Version"]; ok && len(serverVer) > 0 {
            log.Printf("Server version: %s", serverVer[0])
        }
    }
    return err
}

client := productv1.NewProductServiceNatsClient(nc,
    productv1.WithClientInterceptor(clientLoggingInterceptor),
)
```

Request headers go client to server with `WithOutgoingHeaders` and `IncomingHeaders`. Response headers go server to client with `SetResponseHeaders` and `ResponseHeaders`.

## Errors

Generated checkers match the status the handler returned:

```go
product, err := client.GetProduct(ctx, productv1.GetProductRequest_builder{
    Id: proto.String("123"),
}.Build())
if productv1.IsProductServiceNotFound(err) {
    // missing product
}
```

On the server, prefer a generated constructor. A plain `error` becomes `INTERNAL`.

```go
return nil, productv1.NewProductServiceNotFoundError("GetProduct", "not found")
```

Built-in codes: `INVALID_ARGUMENT`, `NOT_FOUND`, `ALREADY_EXISTS`, `PERMISSION_DENIED`, `UNAUTHENTICATED`, `INTERNAL`, `UNAVAILABLE`.

Declare more codes on the service:

```protobuf
service OrderService {
  option (natsmicro.service) = {
    error_codes: ["ORDER_EXPIRED", "PAYMENT_FAILED"]
  };
}
```

```go
return nil, orderv1.NewOrderServiceOrderExpiredError("CreateOrder", "expired after 30m")

if orderv1.IsOrderServiceOrderExpired(err) {
    log.Println("Order expired, please resubmit")
}
```

A hand-written error can implement `NatsErrorCode()`, `NatsErrorMessage()`, and `NatsErrorData()`.

## Streaming, KV, and chunked payloads

Streaming RPCs have typed helpers in Go. The guides are:

- [Streaming RPC](docs/guide/streaming.md)
- [KV and Object Store](docs/guide/kv-object-store.md)
- [Error handling](docs/guide/error-handling.md)
- [Interceptors](docs/guide/interceptors.md)

`chunked_io` on a streaming method generates download helpers and, for a client-streaming bytes field, Go upload helpers (`SendBytes`, `SendReader`, `SendFile`). For large blobs, use a streaming RPC with one `bytes` field. Object Store is for persisting a finished message, not for replacing that stream.

KV and Object Store options on an RPC tell the generated server to write the response into JetStream. Write modes include last-write-wins, compare-and-set, and create-only.

## Examples

| Directory | What it shows |
| --- | --- |
| [`examples/complex-go`](examples/complex-go) | Several services, interceptors, headers, errors. `go run server.go` and `go run client.go`. |
| [`examples/streaming-go`](examples/streaming-go) | Unary, server-streaming, client-streaming, and bidi in Go. |
| [`examples/kvstore-go`](examples/kvstore-go) | KV and Object Store persistence. |
| [`examples/simple-ts`](examples/simple-ts) | TypeScript client and server. Needs a protobuf-ts build that accepts the schema edition. |
| [`examples/simple-py`](examples/simple-py) | Python client and server. Needs `protoc` 36.2 or newer. |

Schemas live in [`examples/protos`](examples/protos). NATS options live in [`extensions/proto/natsmicro/options.proto`](extensions/proto/natsmicro/options.proto).

## Development

```
.
├── extensions/proto/natsmicro/options.proto
├── examples/protos/                 # edition 2026 schemas
├── examples/complex-go/
├── examples/streaming-go/
├── examples/kvstore-go/
├── examples/simple-ts/
├── examples/simple-py/
├── gen/nats/micro/options.pb.go     # generated opaque options, checked in
├── tools/protoc-gen-nats-micro/
│   ├── main.go
│   └── generator/templates/
├── buf.yaml
├── buf.lock
└── Taskfile.yml
```

The plugin reads `options.proto` as generated Go, so extension types are produced first:

```bash
task generate:extensions   # protoc-gen-go over options.proto
task build:plugin          # compile protoc-gen-nats-micro
task generate:go           # example Go services
task test                  # go test ./...
```

`task generate` also runs the TypeScript and Python generators and continues if one of them fails. Use `task generate:go` when you only want Go.

Other tasks: `task build:go`, `task nats`, `task run:go:server`, `task run:go:client`, `task clean`. `task --list` prints the rest.

To add a language, add a template directory and register it in `generator/generator.go`. See [tools/protoc-gen-nats-micro/README.md](tools/protoc-gen-nats-micro/README.md).

## Contributing

Language templates, interceptor examples, and benchmarks are welcome.

## Related projects

- [nats.go](https://github.com/nats-io/nats.go) and [nats.go/micro](https://github.com/nats-io/nats.go/tree/main/micro)
- [buf](https://buf.build)
- [protobuf-go](https://github.com/protocolbuffers/protobuf-go)
- [grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway)

## License

MIT. See [LICENSE](LICENSE).

## Author

Upstream project by [Helba](https://helba.ai). This fork is maintained by [franchb](https://github.com/franchb).
