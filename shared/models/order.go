package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OrderStatus string

const (
	OrderStatusCreated       OrderStatus = "CREATED"
	OrderStatusValidated     OrderStatus = "VALIDATED"
	OrderStatusProcessing    OrderStatus = "PROCESSING"
	OrderStatusPaid          OrderStatus = "PAID"
	OrderStatusShipping      OrderStatus = "SHIPPING"
	OrderStatusDelivered     OrderStatus = "DELIVERED"
	OrderStatusCancelled     OrderStatus = "CANCELLED"
	OrderStatusOutOfStock    OrderStatus = "OUT_OF_STOCK"
	OrderStatusPaymentFailed OrderStatus = "PAYMENT_FAILED"
)

type OrderItem struct {
	ProductId string  `json:"product_id"`
	Quantity  int32   `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
}

type Order struct {
	Id              primitive.ObjectID `bson:"_id" json:"id"`
	CustomerId      string             `json:"customer_id"`
	Items           []OrderItem        `json:"items"`
	Status          OrderStatus        `json:"status"`
	ShippingAddress string             `json:"shipping_address"`
	TotalAmount     float64            `json:"total_amount"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

type Product struct {
	Id          primitive.ObjectID `bson:"_id" json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	UnitPrice   float64            `json:"unit_price"`
	StockLevel  int                `json:"stock_level"`
}

type Customer struct {
	Id             primitive.ObjectID `bson:"_id" json:"id"`
	Name           string             `json:"name"`
	Email          string             `json:"email"`
	PaymentDetails PaymentDetails     `json:"payment_details"`
}

type PaymentDetails struct {
	CardNumber string `json:"card_number"`
	ExpiryDate string `json:"expiry_date"`
	CVV        string `json:"cvv"`
}
