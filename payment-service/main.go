package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/kanishkmehta29/order-management-system/payment-service/service"
	"github.com/kanishkmehta29/order-management-system/shared/config"
)

func main() {
	
	err := godotenv.Load("../.env")
	if err != nil {
		err = godotenv.Load(".env")
		if err != nil{
		log.Printf("Warning: error loading env: %v\n", err)
		}
	}

	kafkaBrokers := []string{os.Getenv("KAFKA_BROKER")}

	mongoClient, err := config.ConnectMongoDB()
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoClient.Disconnect(context.Background())

	// Initialize the payment service
	paymentService := service.NewPaymentService(mongoClient, kafkaBrokers)

	// Start processing validated orders
	ctx, cancel := context.WithCancel(context.Background())
	go paymentService.ProcessValidatedOrders(ctx)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down Payment Service...")

	cancel() // Cancel the context to stop processing
	log.Println("Payment Service stopped")
}
