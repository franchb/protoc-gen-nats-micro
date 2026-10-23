package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	streamv1 "example/gen/streaming/v1"
)

func main() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()
	log.Println("✓ Connected to NATS")

	client := streamv1.NewStreamDemoServiceNatsClient(nc)
	ctx := context.Background()

	// ── 1. Unary: Ping ──────────────────────────────────────────────────
	log.Println("\n── Ping (unary) ──")
	pingResp, err := client.Ping(ctx, streamv1.PingRequest_builder{Payload: proto.String("hello")}.Build())
	if err != nil {
		log.Fatalf("Ping failed: %v", err)
	}
	log.Printf("  ← %s (ts=%d)", pingResp.GetPayload(), pingResp.GetTimestamp())

	// ── 2. Server-streaming: CountUp ────────────────────────────────────
	log.Println("\n── CountUp (server-streaming) ──")
	countStream, err := client.CountUp(ctx, streamv1.CountUpRequest_builder{Start: proto.Int32(1), Count: proto.Int32(5)}.Build())
	if err != nil {
		log.Fatalf("CountUp failed: %v", err)
	}
	for {
		resp, err := countStream.Recv(ctx)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Fatalf("CountUp recv failed: %v", err)
		}
		log.Printf("  ← number=%d ts=%s", resp.GetNumber(), resp.GetTimestamp())
	}
	countStream.Close()
	log.Println("  ✓ Stream complete")

	// ── 3. Client-streaming: Sum ────────────────────────────────────────
	log.Println("\n── Sum (client-streaming) ──")
	sumStream, err := client.Sum(ctx)
	if err != nil {
		log.Fatalf("Sum failed: %v", err)
	}
	values := []int64{10, 20, 30, 40, 50}
	for _, v := range values {
		log.Printf("  → sending %d", v)
		if err := sumStream.Send(streamv1.SumRequest_builder{Value: proto.Int64(v)}.Build()); err != nil {
			log.Fatalf("Sum send failed: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	sumResp, err := sumStream.CloseAndRecv(ctx)
	if err != nil {
		log.Fatalf("Sum close failed: %v", err)
	}
	log.Printf("  ← total=%d count=%d", sumResp.GetTotal(), sumResp.GetCount())

	// ── 4. Bidirectional streaming: Chat ────────────────────────────────
	log.Println("\n── Chat (bidi-streaming) ──")
	chatStream, err := client.Chat(ctx)
	if err != nil {
		log.Fatalf("Chat failed: %v", err)
	}

	messages := []string{"hello", "how are you", "goodbye"}
	for _, text := range messages {
		log.Printf("  → [client] %s", text)
		if err := chatStream.Send(streamv1.ChatMessage_builder{
			User:      proto.String("client"),
			Text:      proto.String(text),
			Timestamp: proto.String(time.Now().Format(time.RFC3339Nano)),
		}.Build()); err != nil {
			log.Fatalf("Chat send failed: %v", err)
		}

		// Read echo back
		reply, err := chatStream.Recv(ctx)
		if err != nil {
			log.Printf("  ✗ recv error: %v", err)
			break
		}
		log.Printf("  ← [%s] %s", reply.GetUser(), reply.GetText())
	}
	chatStream.CloseSend()
	log.Println("  ✓ Chat complete")

	fmt.Println("\n✅ All streaming demos completed successfully!")
}
