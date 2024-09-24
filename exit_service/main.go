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
	"os"
)

type ExitEvent struct {
	ID           string `json:"id"`
	VehiclePlate string `json:"vehicle_plate"`
	ExitDateTime string `json:"exit_date_time"`
}

var ctx = context.Background()

// Simulate exit event generation with 80% matching entry and 20% random plates
// Parameters:
// - rdb: The Redis client.
func generateExitEvent(rdb *redis.Client) ExitEvent {
	var vehiclePlate string
	// 80% chance to generate matching vehicle plate
	if rand.Float32() < 0.8 {
		keys, err := rdb.Keys(ctx, "*").Result()
		if err == nil && len(keys) > 0 {
			vehiclePlate = keys[rand.Intn(len(keys))]
		} else {
			vehiclePlate = fmt.Sprintf("ABC%d", rand.Intn(999))
		}
	} else {
		// 20% chance to generate a random new vehicle plate (no entry in Redis)
		vehiclePlate = fmt.Sprintf("XYZ%d", rand.Intn(999))
	}

	return ExitEvent{
		ID:           fmt.Sprintf("exit-%d", rand.Intn(100000)),
		VehiclePlate: vehiclePlate,
		ExitDateTime: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

// Function to publish event to RabbitMQ
// Parameters:
// - conn: The RabbitMQ connection.
// - queueName: The name of the queue to send the event to.
// - event: The ExitEvent to send.
func sendToQueue(conn *amqp.Connection, queueName string, event ExitEvent) error {
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

	fmt.Printf(" [x] Sent Exit Event: %s\n", body)
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
		// Generate a random exit event
		exitEvent := generateExitEvent(rdb)

		// Send event to RabbitMQ exit queue
		err := sendToQueue(conn, "ExitQueue", exitEvent)
		if err != nil {
			fmt.Println("Failed to send exit event:", err)
		}

		// Sleep for 5 seconds to simulate time between events
		time.Sleep(5 * time.Second)
	}
}
