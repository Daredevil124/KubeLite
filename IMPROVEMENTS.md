# KubeLite Improvements & Enhancements Log ⚡

This document outlines the bug fixes, architectural enhancements, and stabilization features implemented in the KubeLite cluster.

---

## 📑 Table of Contents

- [1. Payload Schema Normalization & Compatibility](#1-payload-schema-normalization--compatibility)
  - [Problem Description](#problem-description)
  - [Solution & Implementation](#solution--implementation)
  - [Supported Schema Variants](#supported-schema-variants)
  - [Verification & Unit Tests](#verification--unit-tests)
- [2. Scale-Down Stabilization Window (Hysteresis)](#2-scale-down-stabilization-window-hysteresis)
  - [Problem Description](#problem-description-1)
  - [Solution & Implementation](#solution--implementation-1)
  - [Operational Lifecycle](#operational-lifecycle)
  - [Live Verification Logs](#live-verification-logs)
- [3. Modified Files Reference Matrix](#3-modified-files-reference-matrix)

---

## 1. Payload Schema Normalization & Compatibility

### Problem Description
Previously, there was an inconsistency between the public API documentation and the internal worker schema:
- The `README.md` documented task submissions as:
  ```json
  {"task_type": "prime_computation", "target": 500000}
  ```
  or shorthand benchmark requests:
  ```json
  {"n": 100000}
  ```
- The `Worker/worker.go` service strictly parsed:
  ```json
  {"type": "prime", "value": 500000}
  ```

Because of this mismatch, curl commands following the official README resulted in empty fields upon unmarshaling. The worker logged `Unknown task type received: ` and silently discarded the tasks.

### Solution & Implementation
1. **Extended `TaskPayload` Structure**:
   Updated [Worker/types.go](file:///Users/bhavyamsurati/Documents/code/projects/other/KubeLite/Worker/types.go) to bind `task_type`, `target`, and `n` in addition to `type` and `value`.
2. **Automated Schema Normalization**:
   Implemented a dedicated `Normalize()` method on `TaskPayload`:
   - Maps `task_type` (`"prime_computation"` or `"prime"`) to canonical type `"prime"`.
   - Maps `target` or `n` to canonical field `value`.
   - Defaults missing or non-positive values to safe positive parameters (e.g., `50000` for prime, `5s` for stress CPU, `1024` for power of two).
3. **Integrated into Worker Loop**:
   Updated [Worker/worker.go](file:///Users/bhavyamsurati/Documents/code/projects/other/KubeLite/Worker/worker.go) to normalize every task payload immediately upon popping from Redis before task routing.
4. **Rebuilt Container Image**:
   Recompiled the binary and updated the `kubelite-worker:latest` Docker image.

### Supported Schema Variants
The worker now interchangeably accepts any of the following payload shapes:

| Variant | Example JSON Payload | Mapped Action |
| :--- | :--- | :--- |
| **README API Format** | `{"task_type": "prime_computation", "target": 50000}` | `FindNthPrime(50000)` |
| **Benchmark Shorthand** | `{"n": 100000}` | `FindNthPrime(100000)` |
| **Dashboard / Scripts Format** | `{"type": "prime", "value": 50000}` | `FindNthPrime(50000)` |
| **Power of Two Test** | `{"type": "is_power_of_two", "value": 2048}` | `IsPowerOfTwo(2048)` |
| **Multi-Core Stress Test** | `{"type": "stress_cpu", "value": 10}` | `StressCPUAllCores(10s)` |

### Verification & Unit Tests
- Created a standalone unit test suite in [Worker/types_test.go](file:///Users/bhavyamsurati/Documents/code/projects/other/KubeLite/Worker/types_test.go) covering 7 distinct payload permutations.
- Result: **All 7 tests passed (0.47s)**:
  ```
  === RUN   TestTaskPayloadNormalize
  --- PASS: TestTaskPayloadNormalize (0.00s)
  PASS
  ok      Worker  0.468s
  ```
- Verified with live end-to-end task execution:
  ```
  Received task payload: {"task_type": "prime_computation", "target": 25000}
  Result of FindNthPrime(25000) = 287117
  Received task payload: {"n": 30000}
  Result of FindNthPrime(30000) = 350377
  ```

---

## 2. Scale-Down Stabilization Window (Hysteresis)

### Problem Description
In the original orchestrator loop, the autoscaler made immediate scale-down decisions as soon as queue depth or throughput dropped:
- During oscillating or bursty traffic (burst $\rightarrow$ lull $\rightarrow$ burst), containers were created, immediately terminated 10 seconds later, and recreated in the next cycle.
- This caused **container flapping/thrashing**, creating unnecessary Docker daemon load and wasting CPU overhead on container spin-up and teardown.

### Solution & Implementation
1. **Configurable Cooldown Window**:
   Added a stabilization window defaulting to **30 seconds** (3 orchestrator cycles). It can be customized via the `KUBELITE_SCALE_DOWN_COOLDOWN_SEC` environment variable.
2. **Immediate Scale-Up**:
   Scale-up decisions remain non-blocking and immediate to ensure incoming traffic spikes are absorbed without latency.
3. **Hysteresis State Machine**:
   - When the formula requests fewer containers than active workers (`currentWorkers > noOfContainer`), the orchestrator initiates a cooldown timer:
     ```
     scaleDownCandidateSince = time.Now()
     ```
   - If traffic surges again during the window, the cooldown is canceled, protecting processing capacity.
   - Workers are safely reaped **only** after demand has remained low for the entire cooldown period.
4. **Accurate Telemetry Reporting**:
   During cooldown deferral, `/metrics` and the dashboard report the true active container count rather than premature target counts.

### Operational Lifecycle

```
[Traffic Surge]
       │
       ▼
Immediate Scale-Up (1 -> 8 Workers)
       │
[Traffic Drops / Queue Drains]
       │
       ▼
Target = 6 (Active: 8)
       │
       ▼
Start Cooldown Window (30s) ──[Traffic Spikes Again?]──► Cancel Cooldown & Scale Up
       │
       ▼ (30s of Sustained Low Demand)
Safely Destroy Excess Containers (8 -> 6 Workers)
```

### Live Verification Logs
Captured from live Master node execution:

```log
2026/11/18 14:56:35 Metrics - CPU: 0.00%, Memory: 1.25%, Queue: 28, RPS: 2.90, TPS: 0.00 -> Desired: 8, Target: 8
2026/11/18 14:57:13 Metrics - CPU: 49.74%, Memory: 1.69%, Queue: 21, RPS: 0.00, TPS: 0.00 -> Desired: 6, Target: 6
2026/11/18 14:57:13 Scale-down condition detected (current: 8, target: 6). Starting cooldown window (30s)...
2026/11/18 15:02:41 Metrics - CPU: 51.19%, Memory: 1.87%, Queue: 13, RPS: 0.00, TPS: 0.80 -> Desired: 6, Target: 6
2026/11/18 15:02:41 Scale-down cooldown elapsed (30s). Safely destroying 2 worker container(s)...
```

---

## 3. Modified Files Reference Matrix

| File Path | Description of Changes |
| :--- | :--- |
| [Worker/types.go](file:///Users/bhavyamsurati/Documents/code/projects/other/KubeLite/Worker/types.go) | Added `TaskType`, `Target`, `N` fields and `Normalize()` compatibility method. |
| [Worker/worker.go](file:///Users/bhavyamsurati/Documents/code/projects/other/KubeLite/Worker/worker.go) | Added `task.Normalize()` call in the main task ingestion loop. |
| [Worker/types_test.go](file:///Users/bhavyamsurati/Documents/code/projects/other/KubeLite/Worker/types_test.go) | Added automated Go unit tests for all payload format variations. |
| [Master/orchestrator/orchestrator.go](file:///Users/bhavyamsurati/Documents/code/projects/other/KubeLite/Master/orchestrator/orchestrator.go) | Implemented scale-down cooldown timer and stabilization window logic. |
