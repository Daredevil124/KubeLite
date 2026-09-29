# KubeLite Test & Formula Benchmark Suite 📊

This directory stores benchmark test runs, raw operational metrics, and generated performance graphs for each autoscaling formula evaluated on the KubeLite engine.

---

## 📁 Directory Structure

```
tests/
├── README.md                           # This guide & protocol specification
│
└── <formula_name>/                     # Folder named after the formula evaluated
    │
    ├── run_YYYY-MM-DD_HH-MM-SS/        # Dedicated folder for EACH run of that formula
    │   ├── metrics_log.csv             # Raw telemetry recorded during test execution
    │   ├── autoscaling_performance.svg # Dual-axis: RPS & Queue depth vs. Worker count
    │   ├── cpu_memory_equilibrium.svg  # Avg CPU % and Memory % vs. Target reference lines
    │   └── formula_definition.txt      # Mathematical breakdown with raw metrics variables
    │
    └── run_.../
```

---

## ⚙️ How to Modify or Add a New Formula

> **CRITICAL RULE**: To modify the formula, edit **ONLY** inside:
>
> 📁 `Master/orchestrator/orchestrator.go` ➔ Function: `CalculateDesiredContainers(...)`
>
> **DO NOT** edit any other part of `orchestrator.go` or the `Orchestrate()` loop.

### Step 1: Open `Master/orchestrator/orchestrator.go`
Locate the function at the bottom:
```go
func CalculateDesiredContainers(
	currentWorkers int64,
	cpuUsage float64,
	memoryUsage float64,
	queueLength int64,
	rps float64,
	tps float64,
) int64 {
	// <-- Write your custom autoscaling logic here -->
	return x
}
```

### Step 2: Available Metrics Reference
Use **only** these metric input arguments:
- `currentWorkers`: Number of active container replicas (`role=worker`).
- `cpuUsage`: Average cluster CPU utilization percentage across all workers.
- `memoryUsage`: Average cluster memory percentage across all workers.
- `queueLength`: Number of pending task payloads in Redis (`task_queue`).
- `rps`: Client HTTP request arrival rate (requests per second).
- `tps`: Cluster task completion rate (tasks per second).

### Step 3: Create a New Formula Folder
If you introduce a new formula, create a new folder under `tests/`:
```bash
mkdir -p tests/<my_new_formula_name>
```

---

## 🚀 How to Run the Load Test & Generate Graphs

### 1. Start the KubeLite Cluster
In **Terminal 1**:
```bash
./scripts/start_kubelite.sh
```

### 2. Run a Benchmark Workload with Automated Logging & Graphing
In **Terminal 2**, run `record_benchmark.sh` followed by the formula folder name and desired test script:

```bash
# Syntax: ./scripts/record_benchmark.sh <formula_folder_name> <test_script_path>

# Example 1: Test Balanced Load
./scripts/record_benchmark.sh balanced_resource_backlog scripts/steady_balanced_load/main.go

# Example 2: Test Queue Backlog Surge
./scripts/record_benchmark.sh balanced_resource_backlog scripts/queue_backlog_spike/main.go

# Example 3: Test Heavy CPU Pegging
./scripts/record_benchmark.sh balanced_resource_backlog scripts/heavy_cpu_spike/main.go
```

When you stop the test with **`CTRL+C`**:
1. The script stops traffic generation and metric polling.
2. It automatically writes `metrics_log.csv` and renders standalone vector graphs (`.svg`) with the embedded mathematical formula definition overlay.
3. The results are saved into `tests/<formula_folder_name>/run_<timestamp>/`.
