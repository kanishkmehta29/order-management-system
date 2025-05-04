package service

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"github.com/kanishkmehta29/order-management-system/shared/kafka"
	"github.com/kanishkmehta29/order-management-system/shared/models"
	kafkago "github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Initialize random seed
func init() {
	rand.Seed(time.Now().UnixNano())
}

type PaymentService struct {
	mongoClient         *mongo.Client
	customersCollection *mongo.Collection
	ordersCollection    *mongo.Collection
	kafkaReader         *kafkago.Reader
	kafkaWriterSuccess  *kafkago.Writer
	kafkaWriterFailed   *kafkago.Writer
	kafkaBrokers        []string
}

// creates a new instance of PaymentService
func NewPaymentService(mongoClient *mongo.Client, kafkaBrokers []string) *PaymentService {
	customersCollection := mongoClient.Database("order_management").Collection("customers")
	ordersCollection := mongoClient.Database("order_management").Collection("orders")

	// Create Kafka reader for inventory-validated topic
	reader := kafka.NewKafkaReader(kafkaBrokers, kafka.TopicInventoryValidated, "payment-service")

	// Create Kafka writers for success and failure events
	successWriter := kafka.NewKafkaWriter(kafkaBrokers, kafka.TopicPaymentProcessed)
	failedWriter := kafka.NewKafkaWriter(kafkaBrokers, kafka.TopicPaymentFailed)

	return &PaymentService{
		mongoClient:         mongoClient,
		customersCollection: customersCollection,
		ordersCollection:    ordersCollection,
		kafkaReader:         reader,
		kafkaWriterSuccess:  successWriter,
		kafkaWriterFailed:   failedWriter,
		kafkaBrokers:        kafkaBrokers,
	}
}

// ProcessValidatedOrders listens for validated orders and processes payments
func (s *PaymentService) ProcessValidatedOrders(ctx context.Context) {
	defer s.kafkaReader.Close()
	defer s.kafkaWriterSuccess.Close()
	defer s.kafkaWriterFailed.Close()

	log.Println("Payment Service is waiting for validated orders...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping payment processing due to context cancellation")
			return
		default:
			// Read message from Kafka
			msg, err := s.kafkaReader.ReadMessage(ctx)
			if err != nil {
				log.Printf("Error reading message: %v", err)
				continue
			}

			// Process the payment
			s.processPayment(ctx, msg)
		}
	}
}

// processPayment handles a validated order and attempts to process payment
func (s *PaymentService) processPayment(ctx context.Context, msg kafkago.Message) {
	var order models.Order
	err := json.Unmarshal(msg.Value, &order)
	if err != nil {
		log.Printf("Failed to unmarshal order: %v", err)
		return
	}

	log.Printf("Processing payment for order: %s", order.Id.Hex())

	// Convert customer ID string to ObjectID
	customerID, err := primitive.ObjectIDFromHex(order.CustomerId)
	if err != nil {
		log.Printf("Invalid customer ID format: %v", err)
		s.handleFailedPayment(ctx, order, "Invalid customer ID")
		return
	}

	// Get customer payment details
	var customer models.Customer
	err = s.customersCollection.FindOne(ctx, bson.M{"_id": customerID}).Decode(&customer)
	if err != nil {
		log.Printf("Failed to find customer: %v", err)
		s.handleFailedPayment(ctx, order, "Customer not found")
		return
	}

	// Simulate payment processing with 80% success rate
	// In a real system, this would integrate with a payment gateway
	paymentSuccessful := simulatePaymentProcessing()

	if paymentSuccessful {
		// Update order status to paid
		_, err := s.ordersCollection.UpdateOne(
			ctx,
			bson.M{"_id": order.Id},
			bson.M{
				"$set": bson.M{
					"status":     models.OrderStatusPaid,
					"updated_at": time.Now(),
				},
			},
		)
		if err != nil {
			log.Printf("Failed to update order status: %v", err)
			return
		}

		// Publish payment-processed event
		order.Status = models.OrderStatusPaid
		order.UpdatedAt = time.Now()
		orderJSON, _ := json.Marshal(order)

		err = kafka.WriteMessage(ctx, s.kafkaWriterSuccess, []byte(order.Id.Hex()), orderJSON)
		if err != nil {
			log.Printf("Failed to publish payment-processed event: %v", err)
		}
	} else {
		s.handleFailedPayment(ctx, order, "Payment gateway declined")
	}
}

// handleFailedPayment updates the order status and publishes a failure event
func (s *PaymentService) handleFailedPayment(ctx context.Context, order models.Order, reason string) {
	// Update order status to payment failed
	_, err := s.ordersCollection.UpdateOne(
		ctx,
		bson.M{"_id": order.Id},
		bson.M{
			"$set": bson.M{
				"status":     models.OrderStatusPaymentFailed,
				"updated_at": time.Now(),
			},
		},
	)
	if err != nil {
		log.Printf("Failed to update order status: %v", err)
		return
	}

	// Publish payment-failed event
	order.Status = models.OrderStatusPaymentFailed
	order.UpdatedAt = time.Now()

	paymentFailedEvent := struct {
		Order  models.Order `json:"order"`
		Reason string       `json:"reason"`
	}{
		Order:  order,
		Reason: reason,
	}

	eventJSON, _ := json.Marshal(paymentFailedEvent)

	err = kafka.WriteMessage(ctx, s.kafkaWriterFailed, []byte(order.Id.Hex()), eventJSON)
	if err != nil {
		log.Printf("Failed to publish payment-failed event: %v", err)
	}
}

// simulatePaymentProcessing simulates payment gateway integration with 80% success rate
func simulatePaymentProcessing() bool {
	// Sleep to simulate processing time
	time.Sleep(time.Millisecond * 500)

	// Return true 80% of the time
	return rand.Float32() < 0.8
}
