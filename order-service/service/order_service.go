package service

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/kanishkmehta29/order-management-system/proto"
	"github.com/kanishkmehta29/order-management-system/shared/kafka"
	"github.com/kanishkmehta29/order-management-system/shared/models"
	kafkago "github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// the OrderService grpc server
type OrderService struct {
	proto.UnimplementedOrderServiceServer  
	mongoClient      *mongo.Client         
	ordersCollection *mongo.Collection    
	kafkaWriter      *kafkago.Writer      
	kafkaBrokers     []string              
}

// creates a new instance of order service
func NewOrderService(mongoClient *mongo.Client, kafkaBrokers []string) *OrderService {
	ordersCollection := mongoClient.Database("order_management").Collection("orders")

	writer := kafka.NewKafkaWriter(kafkaBrokers, kafka.TopicOrderCreated)

	return &OrderService{
		mongoClient:      mongoClient,
		ordersCollection: ordersCollection,
		kafkaWriter:      writer,
		kafkaBrokers:     kafkaBrokers,
	}
}

// implements the CreateOrder rpc method
func (s *OrderService) CreateOrder(ctx context.Context, req *proto.CreateOrderRequest) (*proto.CreateOrderResponse, error) {
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer ID is required")
	}
	if len(req.Items) == 0 {
		return nil, status.Error(codes.InvalidArgument, "order must have at least one item")
	}
	if req.ShippingAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "shipping address is required")
	}

	productsCollection := s.mongoClient.Database("order_management").Collection("products")

	var totalAmount float64
	var orderItems []models.OrderItem

	for _, item := range req.Items {
		var product models.Product
		productID, err := primitive.ObjectIDFromHex(item.ProductId)
		if err != nil {
			log.Printf("Invalid product ID format: %v", err)
			return nil, status.Error(codes.InvalidArgument, "invalid product ID format")
		}

		err = productsCollection.FindOne(ctx, bson.M{"_id": productID}).Decode(&product)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				log.Printf("Product not found: %s", item.ProductId)
				return nil, status.Error(codes.NotFound, "product not found: "+item.ProductId)
			}
			log.Printf("Error finding product: %v", err)
			return nil, status.Error(codes.Internal, "failed to retrieve product information")
		}

		unitPrice := product.UnitPrice
		totalAmount += float64(item.Quantity) * unitPrice

		orderItems = append(orderItems, models.OrderItem{
			ProductId: item.ProductId,
			Quantity:  item.Quantity,
			UnitPrice: unitPrice,
		})
	}

	// create order document
	now := time.Now()
	order := models.Order{
		Id:              primitive.NewObjectID(),
		CustomerId:      req.CustomerId,
		Items:           orderItems,
		Status:          models.OrderStatusCreated,
		ShippingAddress: req.ShippingAddress,
		TotalAmount:     totalAmount,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// insert order into mongodb
	_, err := s.ordersCollection.InsertOne(ctx, order)
	if err != nil {
		log.Printf("Failed to insert order: %v", err)
		return nil, status.Error(codes.Internal, "failed to create order")
	}

	// Publish order created event to Kafka
	orderJSON, err := json.Marshal(order)
	if err != nil {
		log.Printf("Failed to marshal order: %v", err)
		return nil, status.Error(codes.Internal, "failed to process order")
	}

	err = kafka.WriteMessage(ctx, s.kafkaWriter, []byte(order.Id.Hex()), orderJSON)
	if err != nil {
		log.Printf("Failed to publish order created event: %v", err)
	}

	return &proto.CreateOrderResponse{
		OrderId: order.Id.Hex(),
		Status:  string(order.Status),
	}, nil
}

// implements the GetOrder RPC method
func (s *OrderService) GetOrder(ctx context.Context, req *proto.GetOrderRequest) (*proto.Order, error) {
	if req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order ID is required")
	}

	orderID, err := primitive.ObjectIDFromHex(req.OrderId)
	if err != nil {
		log.Printf("Invalid order ID format: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid order ID format")
	}

	var orderModel models.Order
	err = s.ordersCollection.FindOne(ctx, bson.M{"_id": orderID}).Decode(&orderModel)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("Order not found: %s", req.OrderId)
			return nil, status.Error(codes.NotFound, "order not found")
		}
		log.Printf("Error finding order: %v", err)
		return nil, status.Error(codes.Internal, "failed to retrieve order")
	}

	// Convert from models.Order to proto.Order
	protoOrder := &proto.Order{
		Id:              orderModel.Id.Hex(),
		CustomerId:      orderModel.CustomerId,
		Status:          string(orderModel.Status),
		ShippingAddress: orderModel.ShippingAddress,
		TotalAmount:     orderModel.TotalAmount,
		CreatedAt:       orderModel.CreatedAt.Unix(),
	}

	// Convert order items
	for _, item := range orderModel.Items {
		protoOrder.Items = append(protoOrder.Items, &proto.OrderItem{
			ProductId: item.ProductId,
			Quantity:  item.Quantity,
		})
	}

	return protoOrder, nil
}
