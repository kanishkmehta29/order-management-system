package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/kanishkmehta29/order-management-system/order-service/service"
	"github.com/kanishkmehta29/order-management-system/proto"
	"github.com/kanishkmehta29/order-management-system/shared/config"
	"google.golang.org/grpc"
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
		log.Fatalf("error connecting to mongo: %v\n", err)
	}
	defer mongoClient.Disconnect(context.Background())

	grpcServer := grpc.NewServer()
	orderService := service.NewOrderService(mongoClient, kafkaBrokers)
	proto.RegisterOrderServiceServer(grpcServer, orderService)

	// Create a TCP listener
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Start the gRPC server in a goroutine
	go func() {
		log.Println("Starting Order Service on :50051")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down Order Service...")

	grpcServer.GracefulStop()
	log.Println("Order Service stopped")

}
