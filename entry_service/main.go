package main

import (
	"encoding/json"
	"fmt"
	"github.com/streadway/amqp"
	"math/rand"
	"time"
	"log"
	"github.com/go-redis/redis/v8"
	"golang.org/x/net/context"
)

type EntryEvent struct {
	ID            string `json:"id"`
	VehiclePlate  string `json:"vehicle_plate"`
	EntryDateTime string `json:"entry_date_time"` // String to hold formatted datetime
}

var ctx = context.Background()

// Simulate random entry event generation
func generateEntryEvent() EntryEvent {
	return EntryEvent{
		ID:            fmt.Sprintf("entry-%d", rand.Intn(100000)),       // Random unique ID for entry event
		VehiclePlate:  fmt.Sprintf("ABC%d", rand.Intn(999)),            // Random alphanumeric plate
		EntryDateTime: time.Now().UTC().Format(time.RFC3339Nano),       // Format entry time as RFC3339Nano
	}
}

// Store entry in Redis for later retrieval by the exit generator
func storeEntryInRedis(rdb *redis.Client, event EntryEvent) {
	err := rdb.Set(ctx, event.VehiclePlate, event.EntryDateTime, 0).Err()
	if err != nil {
		fmt.Printf("Failed to store Entry Event in Redis: %s\n", err)
	} else {
		fmt.Printf("Stored Entry Event for Vehicle Plate %s\n", event.VehiclePlate)
	}
}

// Send the event to RabbitMQ
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

// Connect to RabbitMQ with retries
func connectRabbitMQ() *amqp.Connection {
	var conn *amqp.Connection
	var err error

	for i := 0; i < 5; i++ { // Retry 5 times
		conn, err = amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
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
	rdb := redis.NewClient(&redis.Options{
		Addr: "redis:6379", // Redis container address
		DB:   0,            // use default DB
	})
	return rdb
}

func main() {
	conn := connectRabbitMQ()
	defer conn.Close()

	rdb := connectRedis()

	log.Println("Successfully connected to RabbitMQ and Redis")

	for {
		// Generate a random entry event
		entryEvent := generateEntryEvent()

		// Store event in Redis
		storeEntryInRedis(rdb, entryEvent)

		// Send event to RabbitMQ entry queue
		err := sendToQueue(conn, "EntryQueue", entryEvent)
		if err != nil {
			fmt.Println("Failed to send entry event:", err)
		}

		// Sleep for 5 seconds to simulate time between events
		time.Sleep(5 * time.Second)
	}
}
