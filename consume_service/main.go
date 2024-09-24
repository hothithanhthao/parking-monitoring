package main

import (
    "log"
    "encoding/json"
    "fmt"
    "net/http"
    "bytes"
    "time"
    "os"
    "github.com/go-redis/redis/v8"
    "github.com/streadway/amqp"
    "golang.org/x/net/context"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var ctx = context.Background()

// Prometheus metrics
var (
    matchedEvents = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "matched_events",
        Help: "Number of matched exit events",
    })
    unmatchedEvents = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "unmatched_events",
        Help: "Number of unmatched exit events",
    })
)

// Structs for Entry and Exit Events
type EntryEvent struct {
    ID            string    `json:"id"`
    VehiclePlate  string    `json:"vehicle_plate"`
    EntryDateTime time.Time `json:"entry_date_time"`
}

type ExitEvent struct {
    ID           string    `json:"id"`
    VehiclePlate string    `json:"vehicle_plate"`
    ExitDateTime time.Time `json:"exit_date_time"`
}

// Struct to log vehicle data via REST API
type VehicleLog struct {
    VehiclePlate  string    `json:"vehicle_plate"`
    EntryDateTime string    `json:"entry_date_time"`
    ExitDateTime  string    `json:"exit_date_time"`
}

// consumeExitQueue consumes messages from the "ExitQueue" in RabbitMQ, processes exit events,
// and matches them with corresponding entry events stored in Redis. If a match is found, it sends
// the vehicle log to a REST API and removes the entry from Redis.
//
// Parameters:
// - conn: The RabbitMQ connection.
// - rdb: The Redis client.
// - apiUrl: The URL of the REST API to send vehicle logs to.
func consumeExitQueue(conn *amqp.Connection, rdb *redis.Client, apiUrl string) {
    ch, err := conn.Channel()
    if err != nil {
        log.Fatalf("Failed to open a channel: %s", err)
    }
    defer ch.Close()

    // Declare the ExitQueue
    queue, err := ch.QueueDeclare(
        "ExitQueue",
        true,
        false,
        false,
        false,
        nil,
    )
    if err != nil {
        log.Fatalf("Failed to declare queue: %s", err)
    }
    log.Printf("Declared queue: %s", queue.Name)

    msgs, err := ch.Consume(
        "ExitQueue",
        "",
        true,
        false,
        false,
        false,
        nil,
    )
    if err != nil {
        log.Fatalf("Failed to register a consumer: %s", err)
    }

    for msg := range msgs {
        var exitEvent ExitEvent
        err := json.Unmarshal(msg.Body, &exitEvent)
        if err != nil {
            fmt.Println("Failed to unmarshal exit event:", err)
            continue
        }

        // Try to find the corresponding entry event in Redis
        entryTimeStr, err := rdb.Get(ctx, exitEvent.VehiclePlate).Result()
        if err != nil {
            // No matching entry event found (20%)
            unmatchedEvents.Inc()
            fmt.Printf("No entry event found for vehicle plate %s (Unmatched)\n", exitEvent.VehiclePlate)
            continue
        }

        // Matching entry event found (80%)
        matchedEvents.Inc()
        fmt.Printf("Found matching entry time for vehicle plate %s\n", exitEvent.VehiclePlate)

        // Parse the entry time from Redis with RFC3339Nano format
        entryTime, err := time.Parse(time.RFC3339Nano, entryTimeStr)
        if err != nil {
            fmt.Printf("Failed to parse entry time for vehicle plate %s: %s\n", exitEvent.VehiclePlate, err)
            continue
        }

        // Prepare vehicle log with RFC3339Nano format for sending to REST API
        vehicleLog := VehicleLog{
            VehiclePlate:  exitEvent.VehiclePlate,
            EntryDateTime: entryTime.UTC().Format(time.RFC3339Nano),
            ExitDateTime:  exitEvent.ExitDateTime.UTC().Format(time.RFC3339Nano),
        }

        sendToRestAPI(vehicleLog, apiUrl)
        fmt.Printf("Processed Exit Event for Vehicle Plate %s\n", exitEvent.VehiclePlate)

        // Remove the entry from Redis
        rdb.Del(ctx, exitEvent.VehiclePlate)
    }
}

// sendToRestAPI sends the vehicle log to a REST API using an HTTP POST request.
//
// Parameters:
// - log: The vehicle log to send.
// - apiUrl: The URL of the REST API.
func sendToRestAPI(log VehicleLog, apiUrl string) {
    jsonData, err := json.Marshal(log)
    if err != nil {
        fmt.Println("Failed to marshal vehicle log:", err)
        return
    }

    resp, err := http.Post(apiUrl+"/save_summary", "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        fmt.Println("Failed to send vehicle log to REST API:", err)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusOK {
        fmt.Println("Vehicle log successfully sent to REST API")
    } else {
        fmt.Println("Failed to log vehicle data via REST API, Status:", resp.Status)
    }
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

// Setup Prometheus Metrics Endpoint
func setupMetrics() {
    prometheus.MustRegister(matchedEvents)
    prometheus.MustRegister(unmatchedEvents)

    http.Handle("/metrics", promhttp.Handler())
    go func() {
        log.Fatal(http.ListenAndServe(":2112", nil)) // Expose metrics on port 2112
    }()
}

func main() {
    setupMetrics()

    conn := connectRabbitMQ()
    defer conn.Close()

    rdb := connectRedis()

    log.Println("Successfully connected to RabbitMQ and Redis")

    apiUrl := os.Getenv("PYTHON_API_URL")
    go consumeExitQueue(conn, rdb, apiUrl)

    select {}
}
