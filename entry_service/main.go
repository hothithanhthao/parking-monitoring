package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/streadway/amqp"
	"golang.org/x/net/context"
)

type EntryEvent struct {
	ID            string `json:"id"`
	VehiclePlate  string `json:"vehicle_plate"`
	EntryDateTime string `json:"entry_date_time"`
}

var ctx = context.Background()

// Simulate random entry event generation
func generateEntryEvent() EntryEvent {
	return EntryEvent{
		ID:            fmt.Sprintf("entry-%d", rand.Intn(100000)),
		VehiclePlate:  fmt.Sprintf("ABC%d", rand.Intn(999)),
		EntryDateTime: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

// Store entry in Redis for later retrieval by the exit generator
// Parameters:
// - rdb: The Redis client.
// - event: The EntryEvent to store.
func storeEntryInRedis(rdb *redis.Client, event EntryEvent) {
	err := rdb.Set(ctx, event.VehiclePlate, event.EntryDateTime, 0).Err()
	if err != nil {
		fmt.Printf("Failed to store Entry Event in Redis: %s\n", err)
	} else {
		fmt.Printf("Stored Entry Event for Vehicle Plate %s\n", event.VehiclePlate)
	}
}

// Send the event to RabbitMQ
// Parameters:
// - conn: The RabbitMQ connection.
// - queueName: The name of the queue to send the event to.
// - event: The EntryEvent to send.
func sendToQueue(conn *amqp.Connection, queueName string, event EntryEvent) error {
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	err = ch.Publish(
		"",
		q.Name,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	if err != nil {
		return err
	}

	fmt.Printf(" [x] Sent Entry Event: %s\n", body)
	return nil
}

// Connect to RabbitMQ with 5 times retry
func connectRabbitMQ() *amqp.Connection {
	rabbitMQHost := os.Getenv("RABBITMQ_HOST")
	rabbitMQPort := os.Getenv("RABBITMQ_PORT")

	if rabbitMQHost == "" || rabbitMQPort == "" {
		log.Fatal("Environment variables RABBITMQ_HOST or RABBITMQ_PORT are not set")
	}

	var conn *amqp.Connection
	var err error

	for i := 0; i < 5; i++ { // Retry 5 times
		conn, err = amqp.Dial(fmt.Sprintf("amqp://guest:guest@%s:%s", rabbitMQHost, rabbitMQPort))
		if err == nil {
			return conn
		}
		log.Printf("Failed to connect to RabbitMQ, retrying in 5 seconds... (%d/5)", i+1)
		time.Sleep(5 * time.Second)
	}

	log.Fatalf("Could not connect to RabbitMQ after 5 attempts: %s", err)
	return nil
}

// Connect to Redis
func connectRedis() *redis.Client {
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")

	if redisHost == "" || redisPort == "" {
		log.Fatal("Environment variables REDIS_HOST or REDIS_PORT are not set")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
		DB:   0,
	})
	return rdb
}

func main() {
	conn := connectRabbitMQ()
	defer conn.Close()

	rdb := connectRedis()

	log.Println("Successfully connected to RabbitMQ and Redis")

	for {
		entryEvent := generateEntryEvent()

		storeEntryInRedis(rdb, entryEvent)

		err := sendToQueue(conn, "EntryQueue", entryEvent)
		if err != nil {
			fmt.Println("Failed to send entry event:", err)
		}

		time.Sleep(5 * time.Second)
	}
}
