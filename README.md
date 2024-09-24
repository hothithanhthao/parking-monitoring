# Event Processing and Monitoring System

This project simulates an event processing system with real-time monitoring using **Prometheus** and **Grafana**. It tracks vehicle entry and exit events in a shopping mall parking system, processing these events through a backend service, and providing detailed metrics for latency and performance visualization.

## Author
-   Thao Ho


## Table of Contents
- [Event Processing and Monitoring System](#event-processing-and-monitoring-system)
  - [Author](#author)
  - [Table of Contents](#table-of-contents)
  - [Description](#description)
  - [Tech Stack](#tech-stack)
  - [Installation](#installation)
    - [1. Clone the Repository](#1-clone-the-repository)
    - [2. Set Up Environment](#2-set-up-environment)
    - [3. Build and Run the Services](#3-build-and-run-the-services)
    - [4. Access Services](#4-access-services)
  - [Testing](#testing)
    - [1. Manually Trigger Event Generation](#1-manually-trigger-event-generation)
    - [2. Test FastAPI Service](#2-test-fastapi-service)
    - [3. Test Metrics Collection](#3-test-metrics-collection)
  - [Visualizing Metrics in Grafana](#visualizing-metrics-in-grafana)
    - [1. Access Grafana](#1-access-grafana)
    - [2. Add Prometheus as a Data Source](#2-add-prometheus-as-a-data-source)
    - [3. Create a New Dashboard](#3-create-a-new-dashboard)

---

## Description

The system consists of multiple services:
- **Entry Service**: Simulates vehicle entry events and sends them to a RabbitMQ queue.
- **Exit Service**: Simulates vehicle exit events (80% matching entry events and 20% unmatched) and sends them to a RabbitMQ queue.
- **Consume Service**: Processes entry and exit events, calculates the duration between entry and exit, and sends a summary to a FastAPI service.
- **FastAPI Service (API)**: Receives and logs vehicle entry/exit data and exposes a `/metrics` endpoint for Prometheus.
- **Prometheus**: Scrapes metrics from the backend services to monitor performance.
- **Grafana**: Visualizes metrics from Prometheus, allowing real-time monitoring of latency, performance, and event statistics.

---

## Tech Stack

- **Language**: Go, Python
- **Messaging**: RabbitMQ
- **Database**: Redis
- **Monitoring**: Prometheus, Grafana
- **Containerization**: Docker, Docker Compose
- **Web Framework**: FastAPI (for Python REST API)

---

## Installation

### 1. Clone the Repository

```bash
git clone https://github.com/your-username/event-processing-system.git
cd event-processing-system
```

### 2. Set Up Environment
Ensure you have Docker and Docker Compose installed on your system.

### 3. Build and Run the Services
Run the following command to build and start all services:
```bash
docker-compose up --build
```
This command will start:

-   RabbitMQ (for event messaging)
-   Redis (for storing vehicle entries)
-   FastAPI service (for logging vehicle data)
-   Entry, Exit, and Consume services
-   Prometheus and Grafana (for monitoring)
-   

### 4. Access Services
-   RabbitMQ Management Console: http://localhost:15672 (default credentials: guest/guest)
-   Prometheus: http://localhost:9090
-   Grafana: http://localhost:3000 (default credentials: admin/admin)
-   FastAPI Service: http://localhost:5000 (API endpoints and metrics exposed here)


## Testing

### 1. Manually Trigger Event Generation
The entry_service and exit_service are automatically triggered to simulate entry and exit events. You can check logs in the container to verify that events are being generated and processed.

For example:
```bash
docker logs entry_service
docker logs exit_service
docker logs consume_service
docker logs api
```

### 2. Test FastAPI Service
You can manually test the FastAPI service by sending POST requests with vehicle data:
```bash
curl -X POST "http://localhost:5000/save_summary" \
    -H "Content-Type: application/json" \
    -d '{
        "vehicle_plate": "FIN123",
        "entry_date_time": "2024-09-24T09:00:00Z",
        "exit_date_time": "2024-09-25T10:00:00Z"
    }'
```

### 3. Test Metrics Collection
-   To ensure that Prometheus is scraping metrics, visit:

    ```bash
    http://localhost:5000/metrics
    ```

    You'll see all the metrics exposed by the FastAPI service.

- Query these metrics in Prometheus at:

    ```bash
    http://localhost:9090
    ```

    Sample queries:
    -   matched_events: Number of matched vehicle exit events.
    -   unmatched_events: Number of unmatched vehicle exit events.
    -   event_processing_latency_seconds: Latency of event processing (in seconds).


## Visualizing Metrics in Grafana

Once all services are up and running, you can visualize the metrics in Grafana.

### 1. Access Grafana
Go to http://localhost:3000 in your browser. Use the default login credentials:
-   Username: admin
-   Password: admin

### 2. Add Prometheus as a Data Source
1. On the left side menu, click on the gear icon (Settings) > Data Sources.
2. Click Add Data Source.
3. Select Prometheus from the list of data sources.
4. Set the URL to http://prometheus:9090 and click Save & Test.

### 3. Create a New Dashboard
1. Click the + icon > Dashboard > New Panel.
2. Add a Query for Prometheus metrics.

    Example queries:

    -   Average Latency (5-minute window):
        ```bash
        rate(event_processing_latency_seconds_sum[5m]) / rate(event_processing_latency_seconds_count[5m])
        ```
    -   99th Percentile Latency:
        ```bash
        histogram_quantile(0.99, rate(event_processing_latency_seconds_bucket[5m]))
        ```
3. Select the visualization type (e.g., Graph or Gauge) and configure it as needed.
4. Save the dashboard by clicking the Save icon in the top right corner.