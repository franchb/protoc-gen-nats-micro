package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	streamv1 "example/gen/streaming/v1"
)

// streamService implements streamv1.StreamDemoServiceNats
type streamService struct{}

// Ping — standard unary RPC
func (s *streamService) Ping(ctx context.Context, req *streamv1.PingRequest) (*streamv1.PingResponse, error) {
	log.Printf("✓ Ping: %s", req.GetPayload())
	return streamv1.PingResponse_builder{
		Payload:   proto.String("pong: " + req.GetPayload()),
		Timestamp: proto.Int64(time.Now().Unix()),
	}.Build(), nil
}

// CountUp — server-streaming RPC
// Emits `count` numbers starting from `start`, one at a time.
func (s *streamService) CountUp(ctx context.Context, req *streamv1.CountUpRequest, stream *streamv1.StreamDemoService_CountUp_Stream) error {
	log.Printf("→ CountUp: start=%d count=%d", req.GetStart(), req.GetCount())
	for i := int32(0); i < req.GetCount(); i++ {
		num := req.GetStart() + i
		resp := streamv1.CountUpResponse_builder{
			Number:    proto.Int32(num),
			Timestamp: proto.String(time.Now().Format(time.RFC3339Nano)),
		}.Build()
		if err := stream.Send(resp); err != nil {
			return fmt.Errorf("failed to send number %d: %w", num, err)
		}
		log.Printf("  → sent %d", num)
		time.Sleep(200 * time.Millisecond) // Simulate work
	}
	log.Printf("✓ CountUp complete (%d numbers)", req.GetCount())
	return nil
}

// Sum — client-streaming RPC
// Reads all values from the client and returns the total.
func (s *streamService) Sum(ctx context.Context, stream *streamv1.StreamDemoService_Sum_Stream) (*streamv1.SumResponse, error) {
	log.Printf("→ Sum: waiting for values...")
	var total int64
	var count int32
	for {
		msg, err := stream.Recv(ctx)
		if err != nil {
			// EOF or stream ended
			break
		}
		total += msg.GetValue()
		count++
		log.Printf("  ← received %d (running total: %d)", msg.GetValue(), total)
	}
	log.Printf("✓ Sum complete: total=%d count=%d", total, count)
	return streamv1.SumResponse_builder{Total: proto.Int64(total), Count: proto.Int32(count)}.Build(), nil
}

// Chat — bidirectional streaming RPC
// Echoes back each message the client sends.
func (s *streamService) Chat(ctx context.Context, stream *streamv1.StreamDemoService_Chat_Stream) error {
	log.Printf("→ Chat: session started")
	for {
		msg, err := stream.Recv(ctx)
		if err != nil {
			// Stream ended
			break
		}
		log.Printf("  ← [%s] %s", msg.GetUser(), msg.GetText())

		// Echo back with server prefix
		reply := streamv1.ChatMessage_builder{
			User:      proto.String("server"),
			Text:      proto.String(fmt.Sprintf("echo: %s", msg.GetText())),
			Timestamp: proto.String(time.Now().Format(time.RFC3339Nano)),
		}.Build()
		if err := stream.Send(reply); err != nil {
			return fmt.Errorf("failed to send chat reply: %w", err)
		}
	}
	log.Printf("✓ Chat: session ended")
	return nil
}

func main() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()
	log.Println("✓ Connected to NATS")

	// Register the streaming service
	impl := &streamService{}
	svc, err := streamv1.RegisterStreamDemoServiceHandlers(nc, impl)
	if err != nil {
		log.Fatalf("Failed to register service: %v", err)
	}

	// Print endpoints
	log.Println("\n📡 StreamDemoService Endpoints:")
	for _, ep := range svc.Endpoints() {
		log.Printf("  • %s → %s", ep.Name, ep.Subject)
	}

	log.Println("\n✅ Streaming server running. Press Ctrl+C to stop.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	log.Println("\n✓ Shutting down...")
}
