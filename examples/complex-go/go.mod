module example

go 1.25.3

require (
	github.com/franchb/protoc-gen-nats-micro v0.0.0-00010101000000-000000000000
	github.com/google/uuid v1.6.0
	github.com/nats-io/nats.go v1.50.0
	google.golang.org/genproto/googleapis/api v0.0.0-20260319201613-d00831a3d3e7
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/nats-io/nkeys v0.4.15 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/crypto v0.49.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
)

replace github.com/franchb/protoc-gen-nats-micro => ../../
