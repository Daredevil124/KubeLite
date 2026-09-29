#!/usr/bin/env bash
set -e

# Resolve repository root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=========================================================="
echo "           ⚡ Starting KubeLite System ⚡                  "
echo "=========================================================="

echo "[1/4] Ensuring Redis is running on localhost:6379..."
if [ ! "$(docker ps -q -f name=kubelite-redis)" ]; then
    if [ "$(docker ps -aq -f status=exited -f name=kubelite-redis)" ]; then
        echo "Starting existing kubelite-redis container..."
        docker start kubelite-redis
    else
        echo "Creating and running new kubelite-redis container..."
        docker run -d --name kubelite-redis -p 6379:6379 redis:alpine
    fi
else
    echo "kubelite-redis is already running."
fi

echo "[2/4] Building Worker Docker image (kubelite-worker:latest)..."
cd "$ROOT_DIR/Worker"
docker build -t kubelite-worker:latest .

echo "[3/4] Starting Master Node on :8080..."
cd "$ROOT_DIR/Master"
go run main.go &
MASTER_PID=$!

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
    echo "Clean shutdown complete."
    exit 0
}

trap cleanup SIGINT SIGTERM
wait
