#!/usr/bin/env bash
# ==============================================================================
# KubeLite Benchmark Recorder & Visualizer
# Usage: ./scripts/record_benchmark.sh [formula_name] [path_to_workload_script]
# Example: ./scripts/record_benchmark.sh balanced_resource_backlog scripts/steady_balanced_load/main.go
# ==============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Auto-detect ActiveFormulaName from Master/orchestrator/orchestrator.go if not specified
DETECTED_NAME=$(grep -oP 'const ActiveFormulaName\s*=\s*"\K[^"]+' "$ROOT_DIR/Master/orchestrator/orchestrator.go" 2>/dev/null || echo "balanced_resource_backlog")
FORMULA_NAME="${1:-$DETECTED_NAME}"
LOAD_SCRIPT="${2:-$ROOT_DIR/scripts/steady_balanced_load/main.go}"

TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
RUN_DIR="$ROOT_DIR/tests/$FORMULA_NAME/run_$TIMESTAMP"
if ! mkdir -p "$RUN_DIR" 2>/dev/null; then
    echo "❌ Error: Could not create benchmark run directory: $RUN_DIR"
    echo "   This usually happens on Linux if 'tests/' was created by root or Docker."
    echo "   To fix permissions, run:"
    echo "     sudo chown -R $USER:$USER \"$ROOT_DIR/tests\""
    exit 1
fi

CSV_FILE="$RUN_DIR/metrics_log.csv"
FORMULA_FILE="$RUN_DIR/formula_definition.txt"

# 1. Save formula definition text alongside run results
cat << 'EOF' > "$FORMULA_FILE"
================================================================================
KubeLite Autoscaling Formula Benchmark Definition
Evaluated in: Master/orchestrator/orchestrator.go -> CalculateDesiredContainers()
================================================================================

1. Resource Demand (CPU & Memory):
   cpuRatio       = cpuUsage / 70.0
   memRatio       = memoryUsage / 75.0
   resourceDemand = noOfWorkers * max(cpuRatio, memRatio)

2. Traffic Demand (Queue Backlog & Net Velocity):
   trafficDemand  = (queueLength / 5.0)
   if rps > tps:
       trafficDemand += (rps - tps) / 2.0

3. Desired Workers (x):
   x = max(minWorkers, ceil(max(resourceDemand, trafficDemand)))

4. Final System Target (Bounded by Host Safety Ceiling):
   Target = min(x, min(y, z))
   where:
     y = floor((Host_CPUs * 0.8) / 0.5)
     z = floor((Host_RAM_MB * 0.8) / 256.0)
================================================================================
EOF

echo "================================================================================"
echo " 📊 KubeLite Benchmark Session Initialized"
echo " Formula  : $FORMULA_NAME"
echo " Run Dir  : $RUN_DIR"
echo " Workload : $LOAD_SCRIPT"
echo "================================================================================"

# Initialize CSV header
if ! echo "timestamp_sec,cpu_percent,memory_percent,queue_length,worker_count,total_requests,total_tasks,rps,tps" > "$CSV_FILE" 2>/dev/null; then
    echo "❌ Error: Could not initialize metrics CSV file at: $CSV_FILE"
    echo "   Please check write permissions for: $RUN_DIR"
    exit 1
fi

# Pre-flight check: ensure Master node is reachable
if ! curl -s --max-time 3 http://localhost:8080/metrics >/dev/null 2>&1; then
    echo "⚠️  WARNING: Master API (http://localhost:8080/metrics) is not responding!"
    echo "   Please make sure KubeLite is running in another terminal via:"
    echo "     ./scripts/start_kubelite.sh"
    echo "--------------------------------------------------------------------------------"
fi

# Background Poller: queries Master /metrics every 2 seconds and appends to CSV
poll_metrics() {
    local prev_req=0
    local prev_task=0
    local prev_time=$(date +%s)

    while true; do
        sleep 2
        local resp
        resp=$(curl -s --max-time 5 http://localhost:8080/metrics 2>/dev/null || echo "")
        if [ -n "$resp" ] && [ "$resp" != "null" ]; then
            local now=$(date +%s)
            local elapsed=$((now - prev_time))
            if [ "$elapsed" -le 0 ]; then elapsed=1; fi

            # Extract metrics using python standard library (no dependencies needed)
            python3 -c "
import json, sys
try:
    d = json.loads('''$resp''')
    now = $now
    elapsed = $elapsed
    prev_req = $prev_req
    prev_task = $prev_task

    cur_req = d.get('total_requests', 0)
    cur_task = d.get('total_tasks', 0)
    rps = max(0.0, float(cur_req - prev_req) / float(elapsed))
    tps = max(0.0, float(cur_task - prev_task) / float(elapsed))

    row = [
        str(now),
        f\"{d.get('cpu_percent', 0.0):.2f}\",
        f\"{d.get('memory_percent', 0.0):.2f}\",
        str(d.get('queue_length', 0)),
        str(d.get('worker_count', 0)),
        str(cur_req),
        str(cur_task),
        f\"{rps:.2f}\",
        f\"{tps:.2f}\"
    ]
    print(','.join(row))
except Exception:
    pass
" >> "$CSV_FILE" 2>/dev/null || true

            prev_req=$(tail -n 1 "$CSV_FILE" | cut -d',' -f6 2>/dev/null || echo "$prev_req")
            prev_task=$(tail -n 1 "$CSV_FILE" | cut -d',' -f7 2>/dev/null || echo "$prev_task")
            prev_time=$now
        fi
    done
}

# Start background metric sampler
poll_metrics &
POLLER_PID=$!

cleanup() {
    echo ""
    echo "================================================================================"
    echo " Finishing benchmark session... Processing charts..."
    kill $POLLER_PID 2>/dev/null || true
    docker stop kubelite-workload 2>/dev/null || true

    if [ ! -f "$CSV_FILE" ]; then
        echo "⚠️  Error: Metrics CSV file was not found at: $CSV_FILE"
        echo "    No telemetry was recorded. Make sure Master node was running on http://localhost:8080."
        exit 1
    fi

    # Check if any telemetry data rows were collected (more than 1 line)
    local line_count
    line_count=$(wc -l < "$CSV_FILE" 2>/dev/null || echo 0)
    if [ "$line_count" -le 1 ]; then
        echo "⚠️  Warning: CSV file only contains header line (0 metrics recorded)."
        echo "    Master node may not have responded to /metrics polling during the benchmark."
        echo "    Check if Master node was running and healthy."
    fi

    # Run Python chart visualizer
    python3 "$ROOT_DIR/scripts/generate_charts.py" "$CSV_FILE" "$RUN_DIR" "$FORMULA_NAME"

    echo "================================================================================"
    echo " 🎯 Results and Graphs Saved Successfully!"
    echo " Output Directory: $RUN_DIR"
    echo "   • CSV Log  : $CSV_FILE"
    echo "   • Formula  : $FORMULA_FILE"
    echo "   • Chart 1  : $RUN_DIR/1_traffic_throughput.svg (RPS vs TPS)"
    echo "   • Chart 2  : $RUN_DIR/2_queue_backlog.svg (Queue Depth)"
    echo "   • Chart 3  : $RUN_DIR/3_worker_scaling.svg (Container Scaling)"
    echo "   • Chart 4  : $RUN_DIR/4_cpu_utilization.svg (Cluster CPU %)"
    echo "   • Chart 5  : $RUN_DIR/5_memory_utilization.svg (Zoomed RAM %)"
    echo "   • Overview : $RUN_DIR/autoscaling_performance.svg"
    echo "   • Overview : $RUN_DIR/cpu_memory_equilibrium.svg"
    echo "================================================================================"
    exit 0
}

trap cleanup SIGINT SIGTERM

echo "Starting workload generator: $LOAD_SCRIPT"
echo "Press [CTRL+C] when you want to finish the test and generate all charts."
echo "--------------------------------------------------------------------------------"

# Run the selected Go load test workload (native Go or via Docker)
if command -v go >/dev/null 2>&1; then
    go run "$LOAD_SCRIPT" || true
else
    echo "Go not found on host. Running workload generator via Docker (golang:alpine)..."
    REL_SCRIPT="${LOAD_SCRIPT#$ROOT_DIR/}"
    docker rm -f kubelite-workload 2>/dev/null || true
    docker run --rm --name kubelite-workload \
        -v "$ROOT_DIR":/app \
        -v "$HOME/go/pkg/mod":/go/pkg/mod \
        -w /app \
        --network container:kubelite-redis \
        golang:alpine go run "$REL_SCRIPT" || true
fi
cleanup
