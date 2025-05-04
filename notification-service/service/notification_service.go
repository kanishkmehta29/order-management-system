// notification-service/service/notification_service.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/kanishkmehta29/order-management-system/shared/kafka"
	"github.com/kanishkmehta29/order-management-system/shared/models"
	kafkago "github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type NotificationService struct {
	mongoClient         *mongo.Client
	customersCollection *mongo.Collection
	ordersCollection    *mongo.Collection
	kafkaReaders        []*kafkago.Reader
	kafkaBrokers        []string
}

// creates a new instance of NotificationService
func NewNotificationService(mongoClient *mongo.Client, kafkaBrokers []string) *NotificationService {
	customersCollection := mongoClient.Database("order_management").Collection("customers")
	ordersCollection := mongoClient.Database("order_management").Collection("orders")

	// Create Kafka readers for all relevant topics
	paymentProcessedReader := kafka.NewKafkaReader(kafkaBrokers, kafka.TopicPaymentProcessed, "notification-service")
	paymentFailedReader := kafka.NewKafkaReader(kafkaBrokers, kafka.TopicPaymentFailed, "notification-service")
	orderStatusReader := kafka.NewKafkaReader(kafkaBrokers, kafka.TopicOrderStatusUpdated, "notification-service")
	inventoryOutOfStockReader := kafka.NewKafkaReader(kafkaBrokers, kafka.TopicInventoryOutOfStock, "notification-service")

	return &NotificationService{
		mongoClient:         mongoClient,
		customersCollection: customersCollection,
		ordersCollection:    ordersCollection,
		kafkaReaders:        []*kafkago.Reader{paymentProcessedReader, paymentFailedReader, orderStatusReader, inventoryOutOfStockReader},
		kafkaBrokers:        kafkaBrokers,
	}
}

// listens for order status events from multiple topics
func (s *NotificationService) ProcessOrderStatusEvents(ctx context.Context) {
	// Close all Kafka readers on function exit
	defer func() {
		for _, reader := range s.kafkaReaders {
			reader.Close()
		}
	}()

	log.Println("Notification Service is waiting for order events...")

	// Create a channel for each reader
	for _, reader := range s.kafkaReaders {
		go func(r *kafkago.Reader) {
			for {
				select {
				case <-ctx.Done():
					log.Println("Stopping notification processing due to context cancellation")
					return
				default:
					// Read message from Kafka
					msg, err := r.ReadMessage(ctx)
					if err != nil {
						log.Printf("Error reading message: %v", err)
						continue
					}

					// Process the notification based on topic
					s.processNotification(ctx, msg, r.Config().Topic)
				}
			}
		}(reader)

		log.Printf("Started listener for topic: %s", reader.Config().Topic)
	}

	// Keep the main function running
	<-ctx.Done()
}

// handles different types of notification events
func (s *NotificationService) processNotification(ctx context.Context, msg kafkago.Message, topic string) {
	log.Printf("Received message from topic: %s", topic)

	switch topic {
	case kafka.TopicPaymentProcessed:
		s.handlePaymentProcessed(ctx, msg)
	case kafka.TopicPaymentFailed:
		s.handlePaymentFailed(ctx, msg)
	case kafka.TopicInventoryOutOfStock:
		s.handleInventoryOutOfStock(ctx, msg)
	case kafka.TopicOrderStatusUpdated:
		s.handleOrderStatusUpdated(ctx, msg)
	default:
		log.Printf("Unknown topic: %s", topic)
	}
}

// sends a notification when payment is successful
func (s *NotificationService) handlePaymentProcessed(ctx context.Context, msg kafkago.Message) {
	var order models.Order
	if err := json.Unmarshal(msg.Value, &order); err != nil {
		log.Printf("Failed to unmarshal order: %v", err)
		return
	}

	// Convert customer ID string to ObjectID
	customerID, err := primitive.ObjectIDFromHex(order.CustomerId)
	if err != nil {
		log.Printf("Invalid customer ID format: %v", err)
		return
	}

	// Get customer information
	var customer models.Customer
	err = s.customersCollection.FindOne(ctx, bson.M{"_id": customerID}).Decode(&customer)
	if err != nil {
		log.Printf("Failed to find customer: %v", err)
		return
	}

	// Send payment confirmation notification
	s.sendNotification(customer, fmt.Sprintf(
		"Payment Confirmed: Your order #%s for $%.2f has been processed. Your items will ship soon!",
		order.Id.Hex(),
		order.TotalAmount,
	))
}

// sends a notification when payment fails
func (s *NotificationService) handlePaymentFailed(ctx context.Context, msg kafkago.Message) {
	var failedEvent struct {
		Order  models.Order `json:"order"`
		Reason string       `json:"reason"`
	}

	if err := json.Unmarshal(msg.Value, &failedEvent); err != nil {
		log.Printf("Failed to unmarshal payment failed event: %v", err)
		return
	}

	// Convert customer ID string to ObjectID
	customerID, err := primitive.ObjectIDFromHex(failedEvent.Order.CustomerId)
	if err != nil {
		log.Printf("Invalid customer ID format: %v", err)
		return
	}

	// Get customer information
	var customer models.Customer
	err = s.customersCollection.FindOne(ctx, bson.M{"_id": customerID}).Decode(&customer)
	if err != nil {
		log.Printf("Failed to find customer: %v", err)
		return
	}

	// Send payment failed notification
	s.sendNotification(customer, fmt.Sprintf(
		"Payment Failed: We were unable to process payment for your order #%s. Reason: %s. Please update your payment method.",
		failedEvent.Order.Id.Hex(),
		failedEvent.Reason,
	))
}

// sends a notification when items are out of stock
func (s *NotificationService) handleInventoryOutOfStock(ctx context.Context, msg kafkago.Message) {
	var outOfStockEvent struct {
		Order           models.Order `json:"order"`
		OutOfStockItems []string     `json:"out_of_stock_items"`
	}

	if err := json.Unmarshal(msg.Value, &outOfStockEvent); err != nil {
		log.Printf("Failed to unmarshal out of stock event: %v", err)
		return
	}

	// Convert customer ID string to ObjectID
	customerID, err := primitive.ObjectIDFromHex(outOfStockEvent.Order.CustomerId)
	if err != nil {
		log.Printf("Invalid customer ID format: %v", err)
		return
	}

	// Get customer information
	var customer models.Customer
	err = s.customersCollection.FindOne(ctx, bson.M{"_id": customerID}).Decode(&customer)
	if err != nil {
		log.Printf("Failed to find customer: %v", err)
		return
	}

	// Send out of stock notification
	s.sendNotification(customer, fmt.Sprintf(
		"Items Out of Stock: We're sorry, but some items in your order #%s are currently out of stock. Our team will contact you shortly.",
		outOfStockEvent.Order.Id.Hex(),
	))
}

// sends a notification for general status updates
func (s *NotificationService) handleOrderStatusUpdated(ctx context.Context, msg kafkago.Message) {
	var order models.Order
	if err := json.Unmarshal(msg.Value, &order); err != nil {
		log.Printf("Failed to unmarshal order: %v", err)
		return
	}

	// Convert customer ID string to ObjectID
	customerID, err := primitive.ObjectIDFromHex(order.CustomerId)
	if err != nil {
		log.Printf("Invalid customer ID format: %v", err)
		return
	}

	// Get customer information
	var customer models.Customer
	err = s.customersCollection.FindOne(ctx, bson.M{"_id": customerID}).Decode(&customer)
	if err != nil {
		log.Printf("Failed to find customer: %v", err)
		return
	}

	// Customize message based on status
	var message string
	switch order.Status {
	case models.OrderStatusShipping:
		message = fmt.Sprintf(
			"Order Shipped: Your order #%s has been shipped! You should receive it within 3-5 business days.",
			order.Id.Hex(),
		)
	case models.OrderStatusDelivered:
		message = fmt.Sprintf(
			"Order Delivered: Your order #%s has been delivered. Thank you for shopping with us!",
			order.Id.Hex(),
		)
	default:
		message = fmt.Sprintf(
			"Order Update: Your order #%s status has been updated to: %s",
			order.Id.Hex(),
			order.Status,
		)
	}

	s.sendNotification(customer, message)
}

// simulates sending a notification to the customer
func (s *NotificationService) sendNotification(customer models.Customer, message string) {
	// Simulate notification delay
	time.Sleep(time.Millisecond * 200)

	log.Printf("NOTIFICATION TO %s <%s>: %s", customer.Name, customer.Email, message)

	// In a real system:
	// - Send email via SendGrid/Mailgun/etc.
	// - Send SMS via Twilio/etc.
}
