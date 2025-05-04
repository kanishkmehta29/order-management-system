package config

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ConnectMongoDB() (*mongo.Client, error) {
	// Try to load env file but don't fail if it's not found
	_ = godotenv.Load(".env")

	mongo_uri := os.Getenv("MONGODB_URI")
	if mongo_uri == "" {
		// Check if we're running in Docker
		_, inDocker := os.LookupEnv("DOCKER_CONTAINER")
		if inDocker {
			mongo_uri = "mongodb://mongodb:27017"
		} else {
			mongo_uri = "mongodb://localhost:27017"
		}
		log.Println("Using default MongoDB URI:", mongo_uri)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(mongo_uri)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Printf("Failed to connect to MongoDB: %v\n", err)
		return nil, err
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Printf("Failed to ping MongoDB: %v\n", err)
		return nil, err
	}

	log.Println("Successfully connected to database")
	return client, nil
}
