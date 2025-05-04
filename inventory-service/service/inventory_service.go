package service

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/kanishkmehta29/order-management-system/shared/kafka"
	"github.com/kanishkmehta29/order-management-system/shared/models"
	kafkago "github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type InventoryService struct {
	mongoClient           *mongo.Client
	productsCollection    *mongo.Collection
	ordersCollection      *mongo.Collection
	kafkaReader           *kafkago.Reader
	kafkaWriterValidated  *kafkago.Writer
	kafkaWriterOutOfStock *kafkago.Writer
	kafkaBrokers          []string
}

// creates a new instance of InventoryService
func NewInventoryService(mongoClient *mongo.Client, kafkaBrokers []string) *InventoryService {
	productsCollection := mongoClient.Database("order_management").Collection("products")
	ordersCollection := mongoClient.Database("order_management").Collection("orders")

	reader := kafka.NewKafkaReader(kafkaBrokers, kafka.TopicOrderCreated, "inventory-service")

	// Create Kafka writers for inventory-validated and inventory-out-of-stock topics
	validatedWriter := kafka.NewKafkaWriter(kafkaBrokers, kafka.TopicInventoryValidated)
	outOfStockWriter := kafka.NewKafkaWriter(kafkaBrokers, kafka.TopicInventoryOutOfStock)

	return &InventoryService{
		mongoClient:           mongoClient,
		productsCollection:    productsCollection,
		ordersCollection:      ordersCollection,
		kafkaReader:           reader,
		kafkaWriterValidated:  validatedWriter,
		kafkaWriterOutOfStock: outOfStockWriter,
		kafkaBrokers:          kafkaBrokers,
	}
}

// ProcessOrders listens for new orders and processes them
func (s *InventoryService) ProcessOrders(ctx context.Context) {
	defer s.kafkaReader.Close()
	defer s.kafkaWriterValidated.Close()
	defer s.kafkaWriterOutOfStock.Close()

	log.Println("Inventory Service is waiting for orders...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping order processing due to context cancellation")
			return
		default:
			// Read message from Kafka
			msg, err := s.kafkaReader.ReadMessage(ctx)
			if err != nil {
				log.Printf("Error reading message: %v", err)
				continue
			}

			// Process the order
			s.handleOrderCreated(ctx, msg)
		}
	}
}

// handleOrderCreated processes an order created event
func (s *InventoryService) handleOrderCreated(ctx context.Context, msg kafkago.Message) {
	var order models.Order
	err := json.Unmarshal(msg.Value, &order)
	if err != nil {
		log.Printf("Failed to unmarshal order: %v", err)
		return
	}

	log.Printf("Processing order: %s", order.Id.Hex())

	// Check stock for all products
	allInStock := true
	outOfStockItems := []string{}

	// Process inventory check
	processingErr := s.processInventory(ctx, &order, &allInStock, &outOfStockItems)

	if processingErr != nil {
		log.Printf("Processing failed: %v", processingErr)

		// Update order status to failed
		updateResult, updateErr := s.ordersCollection.UpdateOne(
			ctx,
			bson.M{"_id": order.Id},
			bson.M{
				"$set": bson.M{
					"status":     models.OrderStatusCancelled,
					"updated_at": time.Now(),
				},
			},
		)
		if updateErr != nil {
			log.Printf("Failed to update order status: %v", updateErr)
		} else {
			log.Printf("Updated order status to CANCELLED: %v", updateResult)
		}

		return
	}

	// Update order status and publish event based on inventory check
	if allInStock {
		// Update order status to validated
		updateResult, err := s.ordersCollection.UpdateOne(
			ctx,
			bson.M{"_id": order.Id},
			bson.M{
				"$set": bson.M{
					"status":     models.OrderStatusValidated,
					"updated_at": time.Now(),
				},
			},
		)
		if err != nil {
			log.Printf("Failed to update order status: %v", err)
			return
		}
		log.Printf("Updated order status to VALIDATED: %v", updateResult)

		// Publish inventory-validated event
		order.Status = models.OrderStatusValidated
		order.UpdatedAt = time.Now()
		orderJSON, _ := json.Marshal(order)

		err = kafka.WriteMessage(ctx, s.kafkaWriterValidated, []byte(order.Id.Hex()), orderJSON)
		if err != nil {
			log.Printf("Failed to publish inventory-validated event: %v", err)
		}
	} else {
		// Update order status to out of stock
		updateResult, err := s.ordersCollection.UpdateOne(
			ctx,
			bson.M{"_id": order.Id},
			bson.M{
				"$set": bson.M{
					"status":     models.OrderStatusOutOfStock,
					"updated_at": time.Now(),
				},
			},
		)
		if err != nil {
			log.Printf("Failed to update order status: %v", err)
			return
		}
		log.Printf("Updated order status to OUT_OF_STOCK: %v", updateResult)

		// Publish inventory-out-of-stock event
		order.Status = models.OrderStatusOutOfStock
		order.UpdatedAt = time.Now()

		// Add out of stock details
		outOfStockEvent := struct {
			Order           models.Order `json:"order"`
			OutOfStockItems []string     `json:"out_of_stock_items"`
		}{
			Order:           order,
			OutOfStockItems: outOfStockItems,
		}

		eventJSON, _ := json.Marshal(outOfStockEvent)

		err = kafka.WriteMessage(ctx, s.kafkaWriterOutOfStock, []byte(order.Id.Hex()), eventJSON)
		if err != nil {
			log.Printf("Failed to publish inventory-out-of-stock event: %v", err)
		}
	}
}

// processInventory handles inventory checks and updates
func (s *InventoryService) processInventory(ctx context.Context, order *models.Order, allInStock *bool, outOfStockItems *[]string) error {
	// First, check if all products are in stock
	for _, item := range order.Items {
		// Convert the product ID string to ObjectID
		productID, err := primitive.ObjectIDFromHex(item.ProductId)
		if err != nil {
			log.Printf("Invalid product ID format: %v", err)
			*outOfStockItems = append(*outOfStockItems, item.ProductId)
			*allInStock = false
			continue
		}

		var product models.Product
		err = s.productsCollection.FindOne(ctx, bson.M{"_id": productID}).Decode(&product)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				log.Printf("Product not found: %s", item.ProductId)
				*outOfStockItems = append(*outOfStockItems, item.ProductId)
				*allInStock = false
				continue
			}
			return err
		}

		// Check if enough stock
		if product.StockLevel < int(item.Quantity) {
			log.Printf("Insufficient stock for product %s: requested %d, available %d",
				item.ProductId, item.Quantity, product.StockLevel)
			*outOfStockItems = append(*outOfStockItems, item.ProductId)
			*allInStock = false
			continue
		}
	}

	// If all products are in stock, update inventory levels
	if *allInStock {
		for _, item := range order.Items {
			productID, _ := primitive.ObjectIDFromHex(item.ProductId)

			// Update stock level
			_, err := s.productsCollection.UpdateOne(
				ctx,
				bson.M{"_id": productID},
				bson.M{"$inc": bson.M{"stock_level": -item.Quantity}},
			)
			if err != nil {
				log.Printf("Failed to update stock for product %s: %v", item.ProductId, err)
				return err
			}
		}
	}

	return nil
}
