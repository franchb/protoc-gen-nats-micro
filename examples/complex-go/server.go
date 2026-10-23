package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	metadatav1 "example/gen/common/metadata/v1"
	typesv1 "example/gen/common/types/v1"
	demov1 "example/gen/demo/v1"
	orderv1 "example/gen/order/v1"
	orderv2 "example/gen/order/v2"
	productv1 "example/gen/product/v1"
)

// Product service implementation
type productService struct {
	products map[string]*productv1.Product
}

func (s *productService) CreateProduct(ctx context.Context, req *productv1.CreateProductRequest) (*productv1.CreateProductResponse, error) {
	id := uuid.New().String()
	now := time.Now()

	product := productv1.Product_builder{
		Id:            proto.String(id),
		Name:          proto.String(req.GetName()),
		Description:   proto.String(req.GetDescription()),
		Sku:           proto.String(req.GetSku()),
		Category:      req.GetCategory().Enum(),
		Price:         req.GetPrice(),
		StockQuantity: proto.Int32(req.GetStockQuantity()),
		ImageUrls:     req.GetImageUrls(),
		Attributes:    req.GetAttributes(),
		Status:        typesv1.Status_STATUS_ACTIVE.Enum(),
		Metadata: metadatav1.Metadata_builder{
			CreatedAt: typesv1.Timestamp_builder{
				Seconds: proto.Int64(now.Unix()),
				Nanos:   proto.Int32(int32(now.Nanosecond())),
			}.Build(),
			UpdatedAt: typesv1.Timestamp_builder{
				Seconds: proto.Int64(now.Unix()),
				Nanos:   proto.Int32(int32(now.Nanosecond())),
			}.Build(),
			CreatedBy: proto.String("system"),
			UpdatedBy: proto.String("system"),
			Tags:      make(map[string]string),
		}.Build(),
	}.Build()
	s.products[id] = product

	log.Printf("✓ Created product: %s (%s) - $%d.%02d", req.GetName(), id, req.GetPrice().GetUnits(), req.GetPrice().GetNanos()/10000000)
	return productv1.CreateProductResponse_builder{Product: product}.Build(), nil
}

func (s *productService) GetProduct(ctx context.Context, req *productv1.GetProductRequest) (*productv1.GetProductResponse, error) {
	product, ok := s.products[req.GetId()]
	if !ok {
		return nil, fmt.Errorf("product not found: %s", req.GetId())
	}
	log.Printf("✓ Retrieved product: %s", product.GetName())
	return productv1.GetProductResponse_builder{Product: product}.Build(), nil
}

func (s *productService) UpdateProduct(ctx context.Context, req *productv1.UpdateProductRequest) (*productv1.UpdateProductResponse, error) {
	product, ok := s.products[req.GetId()]
	if !ok {
		return nil, fmt.Errorf("product not found: %s", req.GetId())
	}

	product.SetName(req.GetName())
	product.SetDescription(req.GetDescription())
	product.SetPrice(req.GetPrice())
	product.SetStockQuantity(req.GetStockQuantity())
	product.SetImageUrls(req.GetImageUrls())
	product.SetAttributes(req.GetAttributes())
	product.GetMetadata().SetUpdatedAt(typesv1.Timestamp_builder{
		Seconds: proto.Int64(time.Now().Unix()),
	}.Build())
	product.GetMetadata().SetUpdatedBy("system")

	log.Printf("✓ Updated product: %s", product.GetName())
	return productv1.UpdateProductResponse_builder{Product: product}.Build(), nil
}

func (s *productService) DeleteProduct(ctx context.Context, req *productv1.DeleteProductRequest) (*productv1.DeleteProductResponse, error) {
	delete(s.products, req.GetId())
	log.Printf("✓ Deleted product: %s", req.GetId())
	return productv1.DeleteProductResponse_builder{Success: proto.Bool(true)}.Build(), nil
}

func (s *productService) SearchProducts(ctx context.Context, req *productv1.SearchProductsRequest) (*productv1.SearchProductsResponse, error) {
	var results []*productv1.Product
	for _, p := range s.products {
		results = append(results, p)
	}
	log.Printf("✓ Search returned %d products", len(results))
	return productv1.SearchProductsResponse_builder{
		Products:   results,
		TotalCount: proto.Int32(int32(len(results))),
	}.Build(), nil
}

// Order service implementation
type orderService struct {
	orders   map[string]*orderv1.Order
	products *productService
}

func (s *orderService) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	id := uuid.New().String()
	now := time.Now()

	var subtotal int64
	for _, item := range req.GetItems() {
		subtotal += item.GetTotalPrice().GetUnits()
	}

	tax := subtotal / 10 // 10% tax
	total := subtotal + tax

	order := orderv1.Order_builder{
		Id:           proto.String(id),
		CustomerId:   proto.String(req.GetCustomerId()),
		CustomerName: proto.String(req.GetCustomerName()),
		Items:        req.GetItems(),
		Subtotal: typesv1.Money_builder{
			CurrencyCode: proto.String("USD"),
			Units:        proto.Int64(subtotal),
		}.Build(),
		Tax: typesv1.Money_builder{
			CurrencyCode: proto.String("USD"),
			Units:        proto.Int64(tax),
		}.Build(),
		Total: typesv1.Money_builder{
			CurrencyCode: proto.String("USD"),
			Units:        proto.Int64(total),
		}.Build(),
		ShippingAddress: req.GetShippingAddress(),
		Status:          typesv1.Status_STATUS_PENDING.Enum(),
		Metadata: metadatav1.Metadata_builder{
			CreatedAt: typesv1.Timestamp_builder{
				Seconds: proto.Int64(now.Unix()),
				Nanos:   proto.Int32(int32(now.Nanosecond())),
			}.Build(),
			UpdatedAt: typesv1.Timestamp_builder{
				Seconds: proto.Int64(now.Unix()),
				Nanos:   proto.Int32(int32(now.Nanosecond())),
			}.Build(),
			CreatedBy: proto.String(req.GetCustomerId()),
			UpdatedBy: proto.String(req.GetCustomerId()),
			Tags:      make(map[string]string),
		}.Build(),
	}.Build()
	s.orders[id] = order

	log.Printf("✓ Created order: %s for %s - $%d.00 (%d items)", id, req.GetCustomerName(), total, len(req.GetItems()))
	return orderv1.CreateOrderResponse_builder{Order: order}.Build(), nil
}

func (s *orderService) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	order, ok := s.orders[req.GetId()]
	if !ok {
		return nil, fmt.Errorf("order not found: %s", req.GetId())
	}
	log.Printf("✓ Retrieved order: %s", order.GetId())
	return orderv1.GetOrderResponse_builder{Order: order}.Build(), nil
}

func (s *orderService) ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	var results []*orderv1.Order
	for _, o := range s.orders {
		if req.GetCustomerId() == "" || o.GetCustomerId() == req.GetCustomerId() {
			results = append(results, o)
		}
	}
	log.Printf("✓ Listed %d orders", len(results))
	return orderv1.ListOrdersResponse_builder{
		Orders:     results,
		TotalCount: proto.Int32(int32(len(results))),
	}.Build(), nil
}

func (s *orderService) UpdateOrderStatus(ctx context.Context, req *orderv1.UpdateOrderStatusRequest) (*orderv1.UpdateOrderStatusResponse, error) {
	order, ok := s.orders[req.GetId()]
	if !ok {
		return nil, fmt.Errorf("order not found: %s", req.GetId())
	}

	order.SetStatus(req.GetStatus())
	order.GetMetadata().SetUpdatedAt(typesv1.Timestamp_builder{
		Seconds: proto.Int64(time.Now().Unix()),
	}.Build())
	order.GetMetadata().SetUpdatedBy("system")

	log.Printf("✓ Updated order %s status to: %v", order.GetId(), req.GetStatus())
	return orderv1.UpdateOrderStatusResponse_builder{Order: order}.Build(), nil
}

// Order service v2 implementation (same logic, different types)
type orderServiceV2 struct {
	orders   map[string]*orderv2.Order
	products *productService
}

func (s *orderServiceV2) CreateOrder(ctx context.Context, req *orderv2.CreateOrderRequest) (*orderv2.CreateOrderResponse, error) {
	id := uuid.New().String()
	now := time.Now()

	var subtotal int64
	for _, item := range req.GetItems() {
		subtotal += item.GetTotalPrice().GetUnits()
	}

	tax := subtotal / 10 // 10% tax
	total := subtotal + tax

	order := orderv2.Order_builder{
		Id:           proto.String(id),
		CustomerId:   proto.String(req.GetCustomerId()),
		CustomerName: proto.String(req.GetCustomerName()),
		Items:        req.GetItems(),
		Subtotal: typesv1.Money_builder{
			CurrencyCode: proto.String("USD"),
			Units:        proto.Int64(subtotal),
		}.Build(),
		Tax: typesv1.Money_builder{
			CurrencyCode: proto.String("USD"),
			Units:        proto.Int64(tax),
		}.Build(),
		Total: typesv1.Money_builder{
			CurrencyCode: proto.String("USD"),
			Units:        proto.Int64(total),
		}.Build(),
		ShippingAddress: req.GetShippingAddress(),
		Status:          typesv1.Status_STATUS_PENDING.Enum(),
		Metadata: metadatav1.Metadata_builder{
			CreatedAt: typesv1.Timestamp_builder{
				Seconds: proto.Int64(now.Unix()),
				Nanos:   proto.Int32(int32(now.Nanosecond())),
			}.Build(),
			UpdatedAt: typesv1.Timestamp_builder{
				Seconds: proto.Int64(now.Unix()),
				Nanos:   proto.Int32(int32(now.Nanosecond())),
			}.Build(),
			CreatedBy: proto.String(req.GetCustomerId()),
			UpdatedBy: proto.String(req.GetCustomerId()),
			Tags:      make(map[string]string),
		}.Build(),
	}.Build()
	s.orders[id] = order

	log.Printf("✓ [V2] Created order: %s for %s - $%d.00 (%d items)", id, req.GetCustomerName(), total, len(req.GetItems()))
	return orderv2.CreateOrderResponse_builder{Order: order}.Build(), nil
}

func (s *orderServiceV2) GetOrder(ctx context.Context, req *orderv2.GetOrderRequest) (*orderv2.GetOrderResponse, error) {
	order, ok := s.orders[req.GetId()]
	if !ok {
		return nil, fmt.Errorf("order not found: %s", req.GetId())
	}
	log.Printf("✓ [V2] Retrieved order: %s", order.GetId())
	return orderv2.GetOrderResponse_builder{Order: order}.Build(), nil
}

func (s *orderServiceV2) ListOrders(ctx context.Context, req *orderv2.ListOrdersRequest) (*orderv2.ListOrdersResponse, error) {
	var results []*orderv2.Order
	for _, o := range s.orders {
		if req.GetCustomerId() == "" || o.GetCustomerId() == req.GetCustomerId() {
			results = append(results, o)
		}
	}
	log.Printf("✓ [V2] Listed %d orders", len(results))
	return orderv2.ListOrdersResponse_builder{
		Orders:     results,
		TotalCount: proto.Int32(int32(len(results))),
	}.Build(), nil
}

func (s *orderServiceV2) UpdateOrderStatus(ctx context.Context, req *orderv2.UpdateOrderStatusRequest) (*orderv2.UpdateOrderStatusResponse, error) {
	order, ok := s.orders[req.GetId()]
	if !ok {
		return nil, fmt.Errorf("order not found: %s", req.GetId())
	}

	order.SetStatus(req.GetStatus())
	order.GetMetadata().SetUpdatedAt(typesv1.Timestamp_builder{
		Seconds: proto.Int64(time.Now().Unix()),
	}.Build())
	order.GetMetadata().SetUpdatedBy("system")

	log.Printf("✓ [V2] Updated order %s status to: %v", order.GetId(), req.GetStatus())
	return orderv2.UpdateOrderStatusResponse_builder{Order: order}.Build(), nil
}

// JSON Demo service implementation
type jsonDemoService struct{}

func (s *jsonDemoService) Echo(ctx context.Context, req *demov1.EchoRequest) (*demov1.EchoResponse, error) {
	log.Printf("✓ [JSON] Echo: %s", req.GetMessage())
	return demov1.EchoResponse_builder{
		Message:   proto.String("JSON Echo: " + req.GetMessage()),
		Timestamp: proto.Int64(time.Now().Unix()),
		Encoding:  proto.String("json"),
	}.Build(), nil
}

func (s *jsonDemoService) GetUser(ctx context.Context, req *demov1.GetUserRequest) (*demov1.GetUserResponse, error) {
	log.Printf("✓ [JSON] GetUser: %s", req.GetId())
	return demov1.GetUserResponse_builder{
		User: demov1.User_builder{
			Id:    proto.String(req.GetId()),
			Name:  proto.String("json_user_" + req.GetId()),
			Email: proto.String("json@example.com"),
			Roles: []string{"user", "admin"},
			Metadata: map[string]string{
				"encoding": "json",
				"service":  "json_service",
			},
		}.Build(),
	}.Build(), nil
}

// Binary Demo service implementation
type binaryDemoService struct{}

func (s *binaryDemoService) Echo(ctx context.Context, req *demov1.EchoRequest) (*demov1.EchoResponse, error) {
	log.Printf("✓ [BINARY] Echo: %s", req.GetMessage())
	return demov1.EchoResponse_builder{
		Message:   proto.String("Binary Echo: " + req.GetMessage()),
		Timestamp: proto.Int64(time.Now().Unix()),
		Encoding:  proto.String("binary"),
	}.Build(), nil
}

func (s *binaryDemoService) GetUser(ctx context.Context, req *demov1.GetUserRequest) (*demov1.GetUserResponse, error) {
	log.Printf("✓ [BINARY] GetUser: %s", req.GetId())
	return demov1.GetUserResponse_builder{
		User: demov1.User_builder{
			Id:    proto.String(req.GetId()),
			Name:  proto.String("binary_user_" + req.GetId()),
			Email: proto.String("binary@example.com"),
			Roles: []string{"user", "tester"},
			Metadata: map[string]string{
				"encoding": "binary",
				"service":  "binary_service",
			},
		}.Build(),
	}.Build(), nil
}

// Example server interceptors

// Product service interceptors - demonstrates reading request headers and setting response headers
func productLoggingInterceptor(ctx context.Context, req interface{}, info *productv1.UnaryServerInfo, handler productv1.UnaryHandler) (interface{}, error) {
	start := time.Now()
	log.Printf("→ [%s.%s] Request started", info.Service, info.Method)

	// Read incoming headers from context
	if headers := productv1.IncomingHeaders(ctx); headers != nil {
		if traceID, ok := headers["X-Trace-Id"]; ok && len(traceID) > 0 {
			log.Printf("  [Headers] Trace-ID: %s", traceID[0])
		}
		if clientVer, ok := headers["X-Client-Version"]; ok && len(clientVer) > 0 {
			log.Printf("  [Headers] Client-Version: %s", clientVer[0])
		}
	}

	// Add response headers
	responseHeaders := nats.Header{}
	responseHeaders.Set("X-Server-Version", "1.0.0")
	responseHeaders.Set("X-Request-Id", fmt.Sprintf("req-%d", time.Now().UnixNano()))
	productv1.SetResponseHeaders(ctx, responseHeaders)

	resp, err := handler(ctx, req)

	duration := time.Since(start)
	if err != nil {
		log.Printf("✗ [%s.%s] Request failed after %v: %v", info.Service, info.Method, duration, err)
	} else {
		log.Printf("✓ [%s.%s] Request completed in %v", info.Service, info.Method, duration)
	}

	return resp, err
}

func productMetricsInterceptor(ctx context.Context, req interface{}, info *productv1.UnaryServerInfo, handler productv1.UnaryHandler) (interface{}, error) {
	start := time.Now()

	resp, err := handler(ctx, req)

	duration := time.Since(start)
	status := "success"
	if err != nil {
		status = "error"
	}

	log.Printf("📊 [METRICS] service=%s method=%s status=%s duration_ms=%d",
		info.Service, info.Method, status, duration.Milliseconds())

	return resp, err
}

func main() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	log.Println("✓ Connected to NATS")

	// Register product service (subject prefix "api.v1" read from proto!)
	// with logging and metrics interceptors
	productSvc := &productService{products: make(map[string]*productv1.Product)}
	productService, err := productv1.RegisterProductServiceHandlers(nc, productSvc,
		productv1.WithServerInterceptor(productLoggingInterceptor),
		productv1.WithServerInterceptor(productMetricsInterceptor),
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("✓ Registered ProductService (with logging + metrics interceptors)")

	// Register order service v1 (subject prefix "api.v1" read from proto!)
	orderSvc := &orderService{
		orders:   make(map[string]*orderv1.Order),
		products: productSvc,
	}
	orderServiceV1, err := orderv1.RegisterOrderServiceHandlers(nc, orderSvc)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("✓ Registered OrderService v1")

	// Register order service v2 (subject prefix "api.v2" read from proto!)
	orderSvcV2 := &orderServiceV2{
		orders:   make(map[string]*orderv2.Order),
		products: productSvc,
	}
	orderServiceV2, err := orderv2.RegisterOrderServiceHandlers(nc, orderSvcV2)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("✓ Registered OrderService v2")

	// Register JSON demo service (subject prefix "demo.json" with JSON encoding!)
	jsonDemoSvc := &jsonDemoService{}
	jsonService, err := demov1.RegisterJSONServiceHandlers(nc, jsonDemoSvc)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("✓ Registered JSONService (JSON encoding)")

	// Register Binary demo service (subject prefix "demo.binary" with binary encoding!)
	binaryDemoSvc := &binaryDemoService{}
	binaryService, err := demov1.RegisterBinaryServiceHandlers(nc, binaryDemoSvc)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("✓ Registered BinaryService (Binary encoding)")

	// Print all service endpoints
	log.Println("\n📡 ProductService Endpoints:")
	for _, ep := range productService.Endpoints() {
		log.Printf("  • %s → %s", ep.Name, ep.Subject)
	}

	log.Println("\n📡 OrderService V1 Endpoints:")
	for _, ep := range orderServiceV1.Endpoints() {
		log.Printf("  • %s → %s", ep.Name, ep.Subject)
	}

	log.Println("\n📡 OrderService V2 Endpoints:")
	for _, ep := range orderServiceV2.Endpoints() {
		log.Printf("  • %s → %s", ep.Name, ep.Subject)
	}

	log.Println("\n📡 JSONService Endpoints (JSON encoding):")
	for _, ep := range jsonService.Endpoints() {
		log.Printf("  • %s → %s", ep.Name, ep.Subject)
	}

	log.Println("\n📡 BinaryService Endpoints (Binary encoding):")
	for _, ep := range binaryService.Endpoints() {
		log.Printf("  • %s → %s", ep.Name, ep.Subject)
	}

	log.Println("\n✅ Server running. Press Ctrl+C to stop.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	log.Println("\n✓ Shutting down...")
}
