package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/kanishkmehta29/order-management-system/shared/models"
)

func main() {
	log.Println("Starting database seeder...")

	// Direct connection to MongoDB in Docker
	mongoURI := "mongodb://mongodb:27017"

	// Create a client with retry options
	clientOptions := options.Client().
		ApplyURI(mongoURI).
		SetServerSelectionTimeout(10 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Printf("Connecting to MongoDB at %s", mongoURI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	// Ping MongoDB to verify connection
	log.Println("Pinging MongoDB...")
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}
	log.Println("Successfully connected to MongoDB!")

	// Get database
	db := client.Database("order_management")

	// Clean and seed collections
	cleanCollections(db)
	seedCustomers(db)
	seedProducts(db)

	log.Println("Database seeding completed successfully!")
}

func cleanCollections(db *mongo.Database) {
	collections := []string{"customers", "products", "orders"}
	for _, collection := range collections {
		log.Printf("Cleaning collection: %s", collection)
		_, err := db.Collection(collection).DeleteMany(context.Background(), bson.M{})
		if err != nil {
			log.Printf("Failed to clean collection %s: %v", collection, err)
		}
	}
}

func seedCustomers(db *mongo.Database) []models.Customer {
	log.Println("Seeding customers...")
	customerCollection := db.Collection("customers")

	customers := []models.Customer{
		{
			Id:    primitive.NewObjectID(),
			Name:  "Raj Sharma",
			Email: "raj.sharma@example.com",
			PaymentDetails: models.PaymentDetails{
				CardNumber: "4111111111111111",
				ExpiryDate: "12/25",
				CVV:        "123",
			},
		},
		{
			Id:    primitive.NewObjectID(),
			Name:  "Priya Patel",
			Email: "priya.patel@example.com",
			PaymentDetails: models.PaymentDetails{
				CardNumber: "5555555555554444",
				ExpiryDate: "10/26",
				CVV:        "456",
			},
		},
		{
			Id:    primitive.NewObjectID(),
			Name:  "Amit Kumar",
			Email: "amit.kumar@example.com",
			PaymentDetails: models.PaymentDetails{
				CardNumber: "378282246310005",
				ExpiryDate: "03/27",
				CVV:        "789",
			},
		},
	}

	for _, customer := range customers {
		_, err := customerCollection.InsertOne(context.Background(), customer)
		if err != nil {
			log.Printf("Failed to insert customer %s: %v", customer.Name, err)
		}
	}

	log.Printf("Inserted %d customers", len(customers))
	return customers
}

func seedProducts(db *mongo.Database) []models.Product {
	log.Println("Seeding products...")
	productCollection := db.Collection("products")

	products := []models.Product{
		{
			Id:          primitive.NewObjectID(),
			Name:        "OnePlus Nord",
			Description: "Mid-range smartphone with 8GB RAM and 128GB storage",
			UnitPrice:   29999.99,
			StockLevel:  50,
		},
		{
			Id:          primitive.NewObjectID(),
			Name:        "Dell Inspiron Laptop",
			Description: "Core i5 laptop with 16GB RAM and 512GB SSD",
			UnitPrice:   65999.99,
			StockLevel:  100,
		},
		{
			Id:          primitive.NewObjectID(),
			Name:        "boAt Rockerz 450",
			Description: "Wireless Bluetooth headphones with 15hr battery life",
			UnitPrice:   1499.99,
			StockLevel:  200,
		},
		{
			Id:          primitive.NewObjectID(),
			Name:        "Noise ColorFit Pro",
			Description: "Smartwatch with SpO2 monitoring and fitness tracking",
			UnitPrice:   2999.99,
			StockLevel:  75,
		},
		{
			Id:          primitive.NewObjectID(),
			Name:        "Samsung Galaxy Tab",
			Description: "10.4-inch tablet with 4GB RAM and 64GB storage",
			UnitPrice:   18999.99,
			StockLevel:  60,
		},
	}

	for _, product := range products {
		_, err := productCollection.InsertOne(context.Background(), product)
		if err != nil {
			log.Printf("Failed to insert product %s: %v", product.Name, err)
		}
	}

	log.Printf("Inserted %d products", len(products))
	return products
}
