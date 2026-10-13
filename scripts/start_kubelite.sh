#!/usr/bin/env bash
set -e

# Resolve repository root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=========================================================="
echo "           ⚡ Starting KubeLite System ⚡                  "
echo "=========================================================="

echo "[1/4] Ensuring Redis is running on localhost:6379 and exposing :8080..."
if [ -z "$(docker port kubelite-redis 8080 2>/dev/null)" ]; then
    echo "Creating and running new kubelite-redis container with port mapping (-p 6379:6379 -p 8080:8080)..."
    docker rm -f kubelite-redis 2>/dev/null || true
    docker run -d --name kubelite-redis -p 6379:6379 -p 8080:8080 redis:alpine
else
    echo "kubelite-redis is already running with port mapping."
fi

echo "[2/4] Building Worker Docker image (kubelite-worker:latest)..."
cd "$ROOT_DIR/Worker"
docker build -t kubelite-worker:latest .

echo "[3/4] Starting Master Node on :8080..."
if command -v go >/dev/null 2>&1; then
    go run main.go &
    MASTER_PID=$!
else
    echo "Go not found on host. Running Master node via Docker (golang:alpine)..."
    docker rm -f kubelite-master 2>/dev/null || true
    mkdir -p "$HOME/go/pkg/mod"
    docker run --rm --name kubelite-master \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v "$ROOT_DIR":/app \
        -v "$HOME/go/pkg/mod":/go/pkg/mod \
        -w /app/Master \
        --network container:kubelite-redis \
        golang:alpine go run main.go &
    MASTER_PID=$!
fi

echo "[4/4] Starting React Frontend Dashboard..."
cd "$ROOT_DIR/Frontend"
if [ ! -d "node_modules" ]; then
    echo "Installing frontend dependencies..."
    npm install
fi
npm run dev &
FRONTEND_PID=$!

echo "=========================================================="
echo "  🚀 KubeLite Cluster is LIVE!"
echo "  📊 Dashboard : http://localhost:5173"
echo "  ⚙️  Master API: http://localhost:8080/metrics"
echo "  Press [CTRL+C] at any time to shutdown all services."
echo "=========================================================="

cleanup() {
    echo ""
    echo "Shutting down KubeLite services..."
    kill $MASTER_PID $FRONTEND_PID 2>/dev/null || true
    docker stop kubelite-master 2>/dev/null || true
    echo "Clean shutdown complete."
    exit 0
}

trap cleanup SIGINT SIGTERM
wait
