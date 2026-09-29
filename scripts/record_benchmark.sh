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
mkdir -p "$RUN_DIR"

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
echo "timestamp_sec,cpu_percent,memory_percent,queue_length,worker_count,total_requests,total_tasks,rps,tps" > "$CSV_FILE"

# Background Poller: queries Master /metrics every 2 seconds and appends to CSV
poll_metrics() {
    local prev_req=0
    local prev_task=0
    local prev_time=$(date +%s)

    while true; do
        sleep 2
        local resp
        resp=$(curl -s --max-time 2 http://localhost:8080/metrics 2>/dev/null || echo "")
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

    # Run Python chart visualizer
    python3 "$ROOT_DIR/scripts/generate_charts.py" "$CSV_FILE" "$RUN_DIR" "$FORMULA_NAME"

    echo "================================================================================"
    echo " 🎯 Results and Graphs Saved Successfully!"
    echo " Output Directory: $RUN_DIR"
    echo "   • CSV Log  : $CSV_FILE"
    echo "   • Formula  : $FORMULA_FILE"
    echo "   • Graph 1  : $RUN_DIR/autoscaling_performance.svg"
    echo "   • Graph 2  : $RUN_DIR/cpu_memory_equilibrium.svg"
    echo "================================================================================"
    exit 0
}

trap cleanup SIGINT SIGTERM

echo "Starting workload generator: $LOAD_SCRIPT"
echo "Press [CTRL+C] when you want to finish the test and generate all charts."
echo "--------------------------------------------------------------------------------"

# Run the selected Go load test workload
go run "$LOAD_SCRIPT" || true
cleanup
