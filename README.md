# KubeLite ⚡

[![Go Version](https://img.shields.io/badge/Go-1.20%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-Engine-2496ED?style=for-the-badge&logo=docker&logoColor=white)](https://www.docker.com/)
[![Redis](https://img.shields.io/badge/Redis-Queue-DC382D?style=for-the-badge&logo=redis&logoColor=white)](https://redis.io/)
[![License](https://img.shields.io/badge/License-MIT-blue?style=for-the-badge)](LICENSE)

**KubeLite** is a high-performance, decoupled **Master-Worker Distributed Task Engine** and **Real-Time Container Metrics Monitor** written in Go.

It provides an event-driven task distribution infrastructure where client workloads are ingested by a Master node, queued in Redis, and consumed by dynamically scalable Docker worker containers. The Master node continuously polls real-time system metrics (CPU utilization, memory usage, queue depth, and request throughput) directly from the Docker Daemon and Redis to optimize task scheduling and drive autoscaling decisions.

---

## 📑 Table of Contents

- [Architectural Overview](#-architectural-overview)
- [End-to-End Task Lifecycle](#-end-to-end-task-lifecycle)
- [Deep Dive: Metrics & Monitoring Engine](#-deep-dive-metrics--monitoring-engine)
  - [1. Cluster CPU Utilization](#1-cluster-cpu-utilization)
  - [2. Memory Utilization (cgroup v1 & v2)](#2-memory-utilization-cgroup-v1--v2)
  - [3. Queue Backlog Monitoring](#3-queue-backlog-monitoring)
  - [4. Request & Task Throughput](#4-request--task-throughput)
- [Repository Blueprint](#-repository-blueprint)
- [Redis Data Schema](#-redis-data-schema)
- [API & Controller Reference](#-api--controller-reference)
- [🚀 Quickstart & Setup Guide](#-quickstart--setup-guide)
- [🧪 Stress Testing & Benchmarking](#-stress-testing--benchmarking)
- [🛣️ Roadmap & Future Enhancements](#-roadmap--future-enhancements)

---

## 🏗️ Architectural Overview

KubeLite uses a decoupled, non-blocking architecture to isolate task ingestion from execution and monitoring:

```
+-----------------------------------------------------------------------------------+
|                                  CLIENT LAYER                                     |
|  +-----------------------------------------------------------------------------+  |
|  |                   HTTP Client / Workload Producer Script                    |  |
|  +---------------------------------------+-------------------------------------+  |
+------------------------------------------|----------------------------------------+
                                           |
                                [ HTTP POST /request ]
                                           |
                                           v
+-----------------------------------------------------------------------------------+
|                                   MASTER NODE                                     |
|  +---------------------------+  +-------------------+  +-----------------------+  |
|  |   HTTP Request Handler    |  |  Atomic Counter   |  |   Metrics Collector   |  |
|  |  (controller/request.go)  |  | (data/counter.go) |  |   (metrics/*.go)      |  |
|  +-------------+-------------+  +-------------------+  +-----------+-----------+  |
+----------------|---------------------------------------------------|--------------+
                 |                                                   |
           (RPush Payload)                                  (Poll Container Stats)
                 |                                                   |
                 v                                                   v
+----------------------------------+               +--------------------------------+
|       REDIS MESSAGE BROKER       |               |          DOCKER DAEMON         |
|   +--------------------------+   |               |   +------------------------+   |
|   |  Queue: "task_queue"     |   |               |   | Containers labeled with|   |
|   |  Counter: "task_completed"|  |               |   |      `role=worker`     |   |
|   +-------------+------------+   |               |   +-----------+------------+   |
+-----------------|----------------+               +---------------|----------------+
                  |                                                |
            (Pop Payload)                                   (Stream Stats)
                  |                                                |
       +----------+--------------------+                           |
       |                               |                           |
       v                               v                           |
+------------------------------+ +------------------------------+  |
|   WORKER CONTAINER #1        | |   WORKER CONTAINER #2        |  |
|   (role=worker)              | |   (role=worker)              |<-+
|   - Task Execution Engine    | |   - Task Execution Engine    |
|   - CPU Stress Test (Prime)  | |   - CPU Stress Test (Prime)  |
+------------------------------+ +------------------------------+
```

---

## 🔄 End-to-End Task Lifecycle

1. **Ingestion**: A client sends a workload payload via HTTP `POST` to the Master Node's `/request` endpoint.
2. **Atomic Tracking**: The Master Node increments an atomic counter (`atomic.AddUint64`) to track incoming requests per second without lock contention.
3. **Enqueue**: The payload is pushed onto the right side of the Redis list `task_queue` via `RPush`.
4. **Deque & Process**: Active worker containers running with label `role=worker` pop payloads from the queue, execute the task (e.g., intensive CPU prime calculations), and increment the completed task count in Redis.
5. **Real-Time Polling**: In parallel, the Master Node periodically inspects Docker container cgroups and Redis statistics to compute total cluster CPU load, memory utilization, queue backlog depth, and throughput.

---

## 📊 Deep Dive: Metrics & Monitoring Engine

The metrics engine in `Master/metrics` directly communicates with the Docker API and Redis client to collect raw operational telemetry and convert it into actionable percentages and totals.

### 1. Cluster CPU Utilization (`metrics/cpu.go`)

Calculates average CPU utilization across all active containers matching the label `role=worker`.

To accurately calculate CPU utilization over a window, KubeLite extracts CPU delta and System CPU delta from Docker container statistics:

$$\text{CPU \%} = \left( \frac{\text{TotalUsage}_t - \text{TotalUsage}_{t-1}}{\text{SystemCPUUsage}_t - \text{SystemCPUUsage}_{t-1}} \right) \times \text{Online CPUs} \times 100$$

- Handles multi-core host environments by factoring in `online_cpus`.
- Averages total CPU percentage across all active worker instances.

### 2. Memory Utilization (`metrics/memory.go`)

Computes real memory usage percentage relative to container memory limits while accounting for memory caching mechanisms:

$$\text{Memory \%} = \left( \frac{\text{Memory Usage} - \text{Cache}}{\text{Memory Limit}} \right) \times 100$$

- **cgroup v2 support**: Reads `inactive_file` from `v.MemoryStats.Stats`.
- **cgroup v1 fallback**: Reads `total_inactive_file` from `v.MemoryStats.Stats`.
- Ensures accurate RSS/working-set memory tracking without cache pollution.

### 3. Queue Backlog Monitoring (`metrics/queueLength.go`)

Queries Redis using the `LLen` command on key `task_queue` to determine pending workload pressure:

```go
length, err := data.RedisClient.LLen(ctx, "task_queue").Result()
```

### 4. Request & Task Throughput (`data/counter.go` & `data/tasks.go`)

- **Incoming Requests**: Thread-safe atomic counter `TotalRequest` modified using `atomic.AddUint64`.
- **Completed Tasks**: Worker nodes increment the Redis key `task_completed` using `Incr`.

---

## 📁 Repository Blueprint

```
KubeLite/
├── Master/
│   ├── controller/
│   │   └── request.go       # HTTP handler for workload ingestion (/request)
│   ├── data/
│   │   ├── counter.go       # Thread-safe atomic request counter
│   │   ├── redis.go         # Redis client initialization & instance handle
│   │   └── tasks.go         # Task completion counter logic in Redis
│   ├── metrics/
│   │   ├── cpu.go           # Cluster CPU percentage calculator via Docker SDK
│   │   ├── docker.go        # Docker client setup & worker filter (label: role=worker)
│   │   ├── memory.go        # Container Memory percentage calculator (cgroup v1/v2)
│   │   └── queueLength.go   # Redis task_queue length retrieval
│   ├── go.mod               # Master module dependencies (Redis v9, Docker SDK)
│   └── go.sum
├── Worker/
│   ├── main.go              # Worker process daemon entry point
│   ├── prime.go             # Trial division prime generator (CPU stress workload)
│   └── go.mod               # Worker module definition
└── README.md
```

---

## 🗄️ Redis Data Schema

| Key Name         | Data Type    | Written By   | Read By      | Purpose                                                 |
| :--------------- | :----------- | :----------- | :----------- | :------------------------------------------------------ |
| `task_queue`     | List         | Master Node  | Worker Nodes | Queue storing task JSON payloads (`RPush` / `LPop`)     |
| `task_completed` | String (Int) | Worker Nodes | Master Node  | Counter tracking total completed tasks (`Incr` / `Get`) |

---

## 🔌 API & Controller Reference

### `POST /request`

Submit a task payload to the queue.

#### Request Headers

```http
Content-Type: application/json
Access-Control-Allow-Origin: *
```

#### Example Request Body

```json
{
  "task_type": "prime_computation",
  "target": 500000
}
```

#### Response Code

- `202 Accepted` - Task successfully queued in Redis.
- `400 Bad Request` - Invalid body stream.
- `500 Internal Server Error` - Redis connection failure.

---

## 🚀 Quickstart & Setup Guide

### 1. Prerequisites

Make sure the following tools are installed on your machine:

- **Go** `v1.20+`
- **Docker Engine** (running locally with socket `/var/run/docker.sock`)
- **Redis Server** (running on `localhost:6379`)

### 2. Start Redis

Launch a local Redis container or daemon:

```bash
docker run -d --name kubelite-redis -p 6379:6379 redis:alpine
```

### 3. Start the Master Node

```bash
cd Master
go run main.go
```

### 4. Deploy Worker Container(s)

Build and run your worker containers, ensuring the container has the label `role=worker`:

```bash
# Build worker image (from Worker directory)
cd Worker
docker build -t kubelite-worker .

# Spawn worker containers with worker role label
docker run -d --label role=worker --name worker-1 kubelite-worker
docker run -d --label role=worker --name worker-2 kubelite-worker
```

---

## 🧪 Stress Testing & Benchmarking

KubeLite includes a built-in CPU stress test workload module (`Worker/prime.go`) using deliberate brute-force trial division (`FindNthPrime`).

To generate a load spike and test the metrics collector:

```bash
# Send 100 concurrent requests pushing heavy computational tasks
for i in {1..100}; do
  curl -X POST http://localhost:8080/request \
    -H "Content-Type: application/json" \
    -d '{"n": 100000}' &
done
```

Observe how:

1. `GetQueueLength()` reports the surge in Redis queue backlog.
2. Worker containers pick up payloads and trigger CPU spikes.
3. `GetTotalClusterCPU()` reflects the heightened CPU load across active worker nodes.

---

## 🛣️ Roadmap & Future Enhancements

- [ ] ⚖️ **Autoscaler Controller**: Implement dynamic worker scaling (spin up/down worker containers based on target CPU percentage and queue backlog depth).
- [ ] ⏱️ **Distributed Timing Wheel Scheduler**: High-precision task scheduling for delayed and recurring jobs.
- [ ] 📊 **Dashboard & Metrics API**: Expose Prometheus endpoints (`/metrics`) and a React dashboard for real-time visual monitoring.
- [ ] 🛡️ **Fault Tolerance & Dead-Letter Queue (DLQ)**: Automatic retries and failed task quarantine.

---
