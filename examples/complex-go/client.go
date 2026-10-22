package main

import (
	"context"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	locationv1 "example/gen/common/location/v1"
	typesv1 "example/gen/common/types/v1"
	demov1 "example/gen/demo/v1"
	orderv1 "example/gen/order/v1"
	orderv2 "example/gen/order/v2"
	productv1 "example/gen/product/v1"
)

// Example client interceptor for request logging - demonstrates sending and reading headers
func clientLoggingInterceptor(ctx context.Context, method string, req, reply interface{}, invoker productv1.UnaryInvoker) error {
	log.Printf("→ [Client] Calling %s", method)
	start := time.Now()

	// Add custom headers for tracing and metadata
	headers := nats.Header{}
	headers.Set("X-Trace-Id", time.Now().Format("trace-20060102150405.000"))
	headers.Set("X-Client-Version", "1.0.0")

	// Add headers to context
	ctx = productv1.WithOutgoingHeaders(ctx, headers)

	err := invoker(ctx, method, req, reply)

	// Read response headers from context
	if respHeaders := productv1.ResponseHeaders(ctx); respHeaders != nil {
		if serverVer, ok := respHeaders["X-Server-Version"]; ok && len(serverVer) > 0 {
			log.Printf("  [Response Headers] Server-Version: %s", serverVer[0])
		}
		if reqId, ok := respHeaders["X-Request-Id"]; ok && len(reqId) > 0 {
			log.Printf("  [Response Headers] Request-ID: %s", reqId[0])
		}
	}

	duration := time.Since(start)
	if err != nil {
		log.Printf("✗ [Client] %s failed after %v: %v", method, duration, err)
	} else {
		log.Printf("✓ [Client] %s completed in %v", method, duration)
	}

	return err
}

func main() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	log.Println("✓ Connected to NATS")

	// Create clients (subject prefixes read from proto!)
	// with client logging interceptor
	productClient := productv1.NewProductServiceNatsClient(nc,
		productv1.WithClientInterceptor(clientLoggingInterceptor),
	)

	// Order client also with interceptor
	orderClientLoggingInterceptor := func(ctx context.Context, method string, req, reply interface{}, invoker orderv1.UnaryInvoker) error {
		log.Printf("→ [OrderClient] Calling %s", method)
		start := time.Now()
		err := invoker(ctx, method, req, reply)
		duration := time.Since(start)
		if err != nil {
			log.Printf("✗ [OrderClient] %s failed after %v: %v", method, duration, err)
		} else {
			log.Printf("✓ [OrderClient] %s completed in %v", method, duration)
		}
		return err
	}

	orderClient := orderv1.NewOrderServiceNatsClient(nc,
		orderv1.WithClientInterceptor(orderClientLoggingInterceptor),
	)

	// Print client endpoints
	log.Println("\n📡 ProductService Client Endpoints:")
	for _, ep := range productClient.Endpoints() {
		log.Printf("  • %s → %s", ep.Name, ep.Subject)
	}

	log.Println("\n📡 OrderService Client Endpoints:")
	for _, ep := range orderClient.Endpoints() {
		log.Printf("  • %s → %s", ep.Name, ep.Subject)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a product
	log.Println("\n→ Creating product...")
	createProdResp, err := productClient.CreateProduct(ctx, productv1.CreateProductRequest_builder{
		Name:        proto.String("Wireless Headphones"),
		Description: proto.String("Premium noise-cancelling wireless headphones"),
		Sku:         proto.String("HEADPHONES-001"),
		Category:    productv1.ProductCategory_CATEGORY_ELECTRONICS.Enum(),
		Price: typesv1.Money_builder{
			CurrencyCode: proto.String("USD"),
			Units:        proto.Int64(299),
			Nanos:        proto.Int32(99 * 10000000),
		}.Build(),
		StockQuantity: proto.Int32(50),
		ImageUrls:     []string{"https://example.com/headphones.jpg"},
		Attributes: map[string]string{
			"color":     "black",
			"bluetooth": "5.0",
		},
	}.Build())
	if err != nil {
		log.Fatalf("CreateProduct failed: %v", err)
	}

	product := createProdResp.GetProduct()
	log.Printf("✓ Created product:")
	log.Printf("  ID:       %s", product.GetId())
	log.Printf("  Name:     %s", product.GetName())
	log.Printf("  Price:    $%d.%02d %s", product.GetPrice().GetUnits(), product.GetPrice().GetNanos()/10000000, product.GetPrice().GetCurrencyCode())
	log.Printf("  Category: %v", product.GetCategory())
	log.Printf("  Stock:    %d units", product.GetStockQuantity())

	// Create an order
	log.Println("\n→ Creating order...")
	createOrderResp, err := orderClient.CreateOrder(ctx, orderv1.CreateOrderRequest_builder{
		CustomerId:   proto.String("customer-123"),
		CustomerName: proto.String("Alice Johnson"),
		Items: []*orderv1.OrderItem{
			orderv1.OrderItem_builder{
				ProductId:   proto.String(product.GetId()),
				ProductName: proto.String(product.GetName()),
				Quantity:    proto.Int32(2),
				UnitPrice:   product.GetPrice(),
				TotalPrice: typesv1.Money_builder{
					CurrencyCode: proto.String("USD"),
					Units:        proto.Int64(product.GetPrice().GetUnits() * 2),
				}.Build(),
			}.Build(),
		},
		ShippingAddress: locationv1.Address_builder{
			Street:  proto.String("123 Main St"),
			City:    proto.String("San Francisco"),
			State:   proto.String("CA"),
			ZipCode: proto.String("94102"),
			Country: proto.String("USA"),
		}.Build(),
	}.Build())
	if err != nil {
		log.Fatalf("CreateOrder failed: %v", err)
	}

	order := createOrderResp.GetOrder()
	log.Printf("✓ Created order:")
	log.Printf("  ID:       %s", order.GetId())
	log.Printf("  Customer: %s", order.GetCustomerName())
	log.Printf("  Items:    %d", len(order.GetItems()))
	log.Printf("  Subtotal: $%d.00", order.GetSubtotal().GetUnits())
	log.Printf("  Tax:      $%d.00", order.GetTax().GetUnits())
	log.Printf("  Total:    $%d.00", order.GetTotal().GetUnits())
	log.Printf("  Status:   %v", order.GetStatus())
	log.Printf("  Address:  %s, %s %s", order.GetShippingAddress().GetCity(), order.GetShippingAddress().GetState(), order.GetShippingAddress().GetZipCode())

	// Update order status
	log.Println("\n→ Updating order status...")
	updateResp, err := orderClient.UpdateOrderStatus(ctx, orderv1.UpdateOrderStatusRequest_builder{
		Id:     proto.String(order.GetId()),
		Status: typesv1.Status_STATUS_ACTIVE.Enum(),
		Reason: proto.String("Payment confirmed"),
	}.Build())
	if err != nil {
		log.Fatalf("UpdateOrderStatus failed: %v", err)
	}
	log.Printf("✓ Order status updated to: %v", updateResp.GetOrder().GetStatus())

	// List orders
	log.Println("\n→ Listing orders...")
	listResp, err := orderClient.ListOrders(ctx, orderv1.ListOrdersRequest_builder{
		CustomerId: proto.String("customer-123"),
	}.Build())
	if err != nil {
		log.Fatalf("ListOrders failed: %v", err)
	}
	log.Printf("✓ Found %d orders for customer", listResp.GetTotalCount())
	for _, o := range listResp.GetOrders() {
		log.Printf("  - Order %s: $%d.00 (%v)", o.GetId(), o.GetTotal().GetUnits(), o.GetStatus())
	}

	// Search products
	log.Println("\n→ Searching products...")
	searchResp, err := productClient.SearchProducts(ctx, productv1.SearchProductsRequest_builder{
		Category: productv1.ProductCategory_CATEGORY_ELECTRONICS.Enum(),
	}.Build())
	if err != nil {
		log.Fatalf("SearchProducts failed: %v", err)
	}
	log.Printf("✓ Found %d products", searchResp.GetTotalCount())
	for _, p := range searchResp.GetProducts() {
		log.Printf("  - %s: $%d.%02d", p.GetName(), p.GetPrice().GetUnits(), p.GetPrice().GetNanos()/10000000)
	}

	// ========== Test Order Service V2 ==========
	log.Println("\n\n========== Testing Order Service V2 ==========")

	// Subject prefix "api.v2" read from proto!
	orderClientV2 := orderv2.NewOrderServiceNatsClient(nc)

	// Print v2 client endpoints
	log.Println("\n📡 OrderService V2 Client Endpoints:")
	for _, ep := range orderClientV2.Endpoints() {
		log.Printf("  • %s → %s", ep.Name, ep.Subject)
	}

	// Create order via v2
	log.Println("\n→ Creating order via v2...")
	createOrderV2Resp, err := orderClientV2.CreateOrder(ctx, orderv2.CreateOrderRequest_builder{
		CustomerId:   proto.String("customer-456"),
		CustomerName: proto.String("Bob Smith"),
		Items: []*orderv2.OrderItem{
			orderv2.OrderItem_builder{
				ProductId:   proto.String(product.GetId()),
				ProductName: proto.String(product.GetName()),
				Quantity:    proto.Int32(1),
				UnitPrice:   product.GetPrice(),
				TotalPrice: typesv1.Money_builder{
					CurrencyCode: proto.String("USD"),
					Units:        proto.Int64(product.GetPrice().GetUnits()),
				}.Build(),
			}.Build(),
		},
		ShippingAddress: locationv1.Address_builder{
			Street:  proto.String("456 Oak Ave"),
			City:    proto.String("New York"),
			State:   proto.String("NY"),
			ZipCode: proto.String("10001"),
			Country: proto.String("USA"),
		}.Build(),
	}.Build())
	if err != nil {
		log.Fatalf("CreateOrder v2 failed: %v", err)
	}

	orderV2 := createOrderV2Resp.GetOrder()
	log.Printf("✓ [V2] Created order:")
	log.Printf("  ID:       %s", orderV2.GetId())
	log.Printf("  Customer: %s", orderV2.GetCustomerName())
	log.Printf("  Total:    $%d.00", orderV2.GetTotal().GetUnits())
	log.Printf("  Status:   %v", orderV2.GetStatus())

	// List v2 orders
	log.Println("\n→ Listing v2 orders...")
	listV2Resp, err := orderClientV2.ListOrders(ctx, orderv2.ListOrdersRequest_builder{
		CustomerId: proto.String("customer-456"),
	}.Build())
	if err != nil {
		log.Fatalf("ListOrders v2 failed: %v", err)
	}
	log.Printf("✓ [V2] Found %d orders for customer", listV2Resp.GetTotalCount())
	for _, o := range listV2Resp.GetOrders() {
		log.Printf("  - Order %s: $%d.00 (%v)", o.GetId(), o.GetTotal().GetUnits(), o.GetStatus())
	}

	// ========== Test Demo Services (JSON vs Binary Encoding) ==========
	log.Println("\n\n========== Testing Demo Services (JSON vs Binary) ==========")

	// Create JSON service client
	jsonClient := demov1.NewJSONServiceNatsClient(nc)
	log.Println("\n📡 JSONService Client Endpoints:")
	for _, ep := range jsonClient.Endpoints() {
		log.Printf("  • %s → %s", ep.Name, ep.Subject)
	}

	// Create Binary service client
	binaryClient := demov1.NewBinaryServiceNatsClient(nc)
	log.Println("\n📡 BinaryService Client Endpoints:")
	for _, ep := range binaryClient.Endpoints() {
		log.Printf("  • %s → %s", ep.Name, ep.Subject)
	}

	// Test JSON service Echo
	log.Println("\n→ Testing JSON service Echo...")
	jsonEchoResp, err := jsonClient.Echo(ctx, demov1.EchoRequest_builder{
		Message:   proto.String("Hello JSON!"),
		Timestamp: proto.Int64(time.Now().Unix()),
	}.Build())
	if err != nil {
		log.Fatalf("JSON Echo failed: %v", err)
	}
	log.Printf("✓ JSON Echo Response:")
	log.Printf("  Message:   %s", jsonEchoResp.GetMessage())
	log.Printf("  Encoding:  %s", jsonEchoResp.GetEncoding())
	log.Printf("  Timestamp: %d", jsonEchoResp.GetTimestamp())

	// Test Binary service Echo
	log.Println("\n→ Testing Binary service Echo...")
	binaryEchoResp, err := binaryClient.Echo(ctx, demov1.EchoRequest_builder{
		Message:   proto.String("Hello Binary!"),
		Timestamp: proto.Int64(time.Now().Unix()),
	}.Build())
	if err != nil {
		log.Fatalf("Binary Echo failed: %v", err)
	}
	log.Printf("✓ Binary Echo Response:")
	log.Printf("  Message:   %s", binaryEchoResp.GetMessage())
	log.Printf("  Encoding:  %s", binaryEchoResp.GetEncoding())
	log.Printf("  Timestamp: %d", binaryEchoResp.GetTimestamp())

	// Test JSON service GetUser
	log.Println("\n→ Testing JSON service GetUser...")
	jsonUserResp, err := jsonClient.GetUser(ctx, demov1.GetUserRequest_builder{
		Id: proto.String("user-123"),
	}.Build())
	if err != nil {
		log.Fatalf("JSON GetUser failed: %v", err)
	}
	log.Printf("✓ JSON GetUser Response:")
	log.Printf("  ID:       %s", jsonUserResp.GetUser().GetId())
	log.Printf("  Name:     %s", jsonUserResp.GetUser().GetName())
	log.Printf("  Email:    %s", jsonUserResp.GetUser().GetEmail())
	log.Printf("  Roles:    %v", jsonUserResp.GetUser().GetRoles())
	log.Printf("  Metadata: %v", jsonUserResp.GetUser().GetMetadata())

	// Test Binary service GetUser
	log.Println("\n→ Testing Binary service GetUser...")
	binaryUserResp, err := binaryClient.GetUser(ctx, demov1.GetUserRequest_builder{
		Id: proto.String("user-456"),
	}.Build())
	if err != nil {
		log.Fatalf("Binary GetUser failed: %v", err)
	}
	log.Printf("✓ Binary GetUser Response:")
	log.Printf("  ID:       %s", binaryUserResp.GetUser().GetId())
	log.Printf("  Name:     %s", binaryUserResp.GetUser().GetName())
	log.Printf("  Email:    %s", binaryUserResp.GetUser().GetEmail())
	log.Printf("  Roles:    %v", binaryUserResp.GetUser().GetRoles())
	log.Printf("  Metadata: %v", binaryUserResp.GetUser().GetMetadata())

	log.Println("\n✅ All tests passed! v1, v2, JSON and Binary APIs all working!")
	log.Println("\n💡 Note: JSON encoding uses human-readable format (larger, slower)")
	log.Println("   Binary encoding uses protobuf binary format (smaller, faster)")
}
