package main

import (
    "log"
    "encoding/json"
    "fmt"
    "github.com/go-redis/redis/v8"
    "github.com/streadway/amqp"
    "golang.org/x/net/context"
    "net/http"
    "bytes"
    "time"
)

var ctx = context.Background()

// Counters for matched and unmatched events
var matchedCount int
var unmatchedCount int

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
    EntryDateTime string    `json:"entry_date_time"` // Send as string in RFC3339Nano format
    ExitDateTime  string    `json:"exit_date_time"`  // Send as string in RFC3339Nano format
}

// Declaring ExitQueue and Consuming Messages
func consumeExitQueue(conn *amqp.Connection, rdb *redis.Client, apiUrl string) {
    ch, err := conn.Channel()
    if err != nil {
        log.Fatalf("Failed to open a channel: %s", err)
    }
    defer ch.Close()

    // Declare the ExitQueue
    queue, err := ch.QueueDeclare(
        "ExitQueue", // queue name
        true,        // durable
        false,       // delete when unused
        false,       // exclusive
        false,       // no-wait
        nil,         // arguments
    )
    if err != nil {
        log.Fatalf("Failed to declare queue: %s", err)
    }
    log.Printf("Declared queue: %s", queue.Name)

    msgs, err := ch.Consume(
        "ExitQueue", // queue
        "",          // consumer
        true,        // auto-ack
        false,       // exclusive
        false,       // no-local
        false,       // no-wait
        nil,         // args
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
            unmatchedCount++
            fmt.Printf("No entry event found for vehicle plate %s (Unmatched)\n", exitEvent.VehiclePlate)
            log.Printf("Unmatched Count: %d, Matched Count: %d\n", unmatchedCount, matchedCount)
            continue
        }

        // Matching entry event found (80%)
        matchedCount++
        fmt.Printf("Found matching entry time for vehicle plate %s\n", exitEvent.VehiclePlate)
        log.Printf("Unmatched Count: %d, Matched Count: %d\n", unmatchedCount, matchedCount)

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

// Send vehicle log to REST API
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
        DB:   0,            // Use default DB
    })
    return rdb
}

func main() {
    conn := connectRabbitMQ()
    defer conn.Close()

    rdb := connectRedis()

    log.Println("Successfully connected to RabbitMQ and Redis")

    // Consume exit events asynchronously
    apiUrl := "http://api:5000"
    go consumeExitQueue(conn, rdb, apiUrl)

    // Keep the service running
    select {}
}
