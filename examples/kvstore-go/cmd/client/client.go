package main

import (
	"context"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"

	kvdemov1 "example/gen/kvstore_demo/v1"
)

func main() {
	// Connect to NATS
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()
	log.Println("✓ Connected to NATS")

	// Create JetStream context (needed for KV/ObjectStore reads)
	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("Failed to create JetStream context: %v", err)
	}

	// Create client WITH JetStream support for KV/ObjectStore convenience reads
	client := kvdemov1.NewKVStoreDemoServiceNatsClient(nc,
		kvdemov1.WithNatsClientJetStream(js), // ← Enables Get*FromKV / Get*FromObjectStore
	)

	// Print endpoints
	log.Println("\n📡 Client Endpoints:")
	for _, ep := range client.Endpoints() {
		log.Printf("  • %s → %s", ep.Name, ep.Subject)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// ========================================================================
	// Step 1: Save a profile via normal RPC
	// The server handler auto-persists the response to KV bucket "user_profiles"
	// with key "user.{id}" — you write ZERO persistence code.
	// ========================================================================
	log.Println("\n→ Step 1: SaveProfile via RPC (server auto-persists to KV)")

	profile, err := client.SaveProfile(ctx, kvdemov1.SaveProfileRequest_builder{
		Id:    proto.String("123"),
		Name:  proto.String("Alice"),
		Email: proto.String("alice@example.com"),
		Bio:   proto.String("Distributed systems engineer"),
	}.Build())
	if err != nil {
		log.Fatalf("SaveProfile failed: %v", err)
	}
	log.Printf("✓ Profile saved:")
	log.Printf("  ID:    %s", profile.GetId())
	log.Printf("  Name:  %s", profile.GetName())
	log.Printf("  Email: %s", profile.GetEmail())

	// ========================================================================
	// Step 2: Read the profile DIRECTLY from KV — no RPC needed!
	// This reads from the NATS KV bucket, bypassing the server entirely.
	// The key matches the key_template pattern from the proto definition.
	// ========================================================================
	log.Println("\n→ Step 2: Read profile directly from KV (no RPC!)")

	cachedProfile, err := client.GetSaveProfileFromKV(ctx, "user.123")
	if err != nil {
		log.Fatalf("GetSaveProfileFromKV failed: %v", err)
	}
	log.Printf("✓ Profile from KV cache:")
	log.Printf("  ID:    %s", cachedProfile.GetId())
	log.Printf("  Name:  %s", cachedProfile.GetName())
	log.Printf("  Email: %s", cachedProfile.GetEmail())
	log.Printf("  Bio:   %s", cachedProfile.GetBio())

	// ========================================================================
	// Step 3: Generate a report via RPC
	// Server auto-persists to Object Store bucket "reports" with key "report.{id}"
	// ========================================================================
	log.Println("\n→ Step 3: GenerateReport via RPC (server auto-persists to Object Store)")

	report, err := client.GenerateReport(ctx, kvdemov1.GenerateReportRequest_builder{
		Id:     proto.String("456"),
		Title:  proto.String("Q4 Revenue Report"),
		Format: proto.String("pdf"),
	}.Build())
	if err != nil {
		log.Fatalf("GenerateReport failed: %v", err)
	}
	log.Printf("✓ Report generated:")
	log.Printf("  ID:    %s", report.GetId())
	log.Printf("  Title: %s", report.GetTitle())
	log.Printf("  Type:  %s", report.GetContentType())
	log.Printf("  Size:  %d bytes", report.GetSizeBytes())

	// ========================================================================
	// Step 4: Read the report DIRECTLY from Object Store — no RPC needed!
	// ========================================================================
	log.Println("\n→ Step 4: Read report directly from Object Store (no RPC!)")

	cachedReport, err := client.GetGenerateReportFromObjectStore(ctx, "report.456")
	if err != nil {
		log.Fatalf("GetGenerateReportFromObjectStore failed: %v", err)
	}
	log.Printf("✓ Report from Object Store cache:")
	log.Printf("  ID:    %s", cachedReport.GetId())
	log.Printf("  Title: %s", cachedReport.GetTitle())
	log.Printf("  Size:  %d bytes", cachedReport.GetSizeBytes())

	// ========================================================================
	// Step 5: Normal RPC without KV — GetProfile is a standard unary RPC
	// No auto-persistence, no convenience methods. Just a normal call.
	// ========================================================================
	log.Println("\n→ Step 5: GetProfile via normal RPC (no KV)")

	gotProfile, err := client.GetProfile(ctx, kvdemov1.GetProfileRequest_builder{Id: proto.String("123")}.Build())
	if err != nil {
		log.Fatalf("GetProfile failed: %v", err)
	}
	log.Printf("✓ Profile from RPC:")
	log.Printf("  ID:    %s", gotProfile.GetId())
	log.Printf("  Name:  %s", gotProfile.GetName())

	log.Println("\n✅ All done! KV and Object Store features working!")
	log.Println("\n💡 Key insight:")
	log.Println("   Steps 1 & 3 used normal RPCs — the server auto-persisted responses.")
	log.Println("   Steps 2 & 4 read directly from NATS stores — ZERO RPC overhead.")
	log.Println("   Step 5 was a normal RPC with no caching — business as usual.")
}
