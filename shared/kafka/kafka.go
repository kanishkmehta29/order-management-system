package kafka

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	TopicOrderCreated        = "order-created"
	TopicInventoryValidated  = "inventory-validated"
	TopicPaymentProcessed    = "payment-processed"
	TopicOrderStatusUpdated  = "order-status-updated"
	TopicInventoryOutOfStock = "inventory-out-of-stock"
	TopicPaymentFailed       = "payment-failed"
)

// NewKafkaWriter creates a new kafka writer with minimal required configuration
func NewKafkaWriter(brokers []string, topic string) *kafka.Writer {
	return &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne, // Only leader acknowledgment needed
	}
}

// NewKafkaReader creates a new Kafka reader with minimal required configuration
func NewKafkaReader(brokers []string, topic string, groupId string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: groupId, //the consumer group identifier
	})
}

// writes a message with provided key and value to Kafka using the specified writer
func WriteMessage(ctx context.Context, writer *kafka.Writer, key []byte, value []byte) error {
	message := kafka.Message{
		Key:   key, // messages with the same key assigned to the same partition
		Value: value,
		Time:  time.Now(),
	}

	err := writer.WriteMessages(ctx, message)
	if err != nil {
		log.Printf("failed to write message to kafka: %v", err)
		return err
	}
	return nil
}

// creates Kafka topics with the specified names, 3 partitions and replication factor 1
func CreateTopics(brokerAddr string, topics []string) error {
	conn, err := kafka.Dial("tcp", brokerAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return err
	}

	controllerConn, err := kafka.Dial("tcp", controller.Host+":"+strconv.Itoa(controller.Port))
	if err != nil {
		return err
	}
	defer controllerConn.Close()

	topicConfigs := make([]kafka.TopicConfig, 0, len(topics))
	for _, topic := range topics {
		topicConfigs = append(topicConfigs, kafka.TopicConfig{
			Topic:             topic,
			NumPartitions:     3,
			ReplicationFactor: 1,
		})
	}

	return controllerConn.CreateTopics(topicConfigs...)
}
