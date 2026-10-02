module example

go 1.27.0

require (
	github.com/franchb/protoc-gen-nats-micro v0.0.0
	github.com/nats-io/nats.go v1.54.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/klauspost/compress v1.20.1 // indirect
	github.com/nats-io/nkeys v0.4.16 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/crypto v0.57.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
)

replace github.com/franchb/protoc-gen-nats-micro => ../../
