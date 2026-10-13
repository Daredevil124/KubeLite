#!/usr/bin/env bash
set -e

# Resolve repository root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
if command -v go >/dev/null 2>&1; then
    MASTER_MODE="host"
else
    MASTER_MODE="docker"
fi

echo "=========================================================="
echo "           ⚡ Starting KubeLite System ⚡                  "
echo "=========================================================="

echo "[1/4] Ensuring Redis is running on localhost:6379..."
if [ "$MASTER_MODE" = "host" ]; then
    if docker port kubelite-redis 8080 >/dev/null 2>&1; then
        echo "Recreating kubelite-redis without host :8080 so the local Master can bind that port..."
        docker rm -f kubelite-redis 2>/dev/null || true
    fi
    if [ -z "$(docker ps -q -f name=^/kubelite-redis$ 2>/dev/null)" ]; then
        echo "Creating and running new kubelite-redis container with port mapping (-p 6379:6379)..."
        docker rm -f kubelite-redis 2>/dev/null || true
        docker run -d --name kubelite-redis -p 6379:6379 redis:alpine
    else
        echo "kubelite-redis is already running."
    fi
else
    echo "Docker Master mode: Redis container also publishes host :8080 for the shared Master network namespace."
    if [ -z "$(docker port kubelite-redis 8080 2>/dev/null)" ]; then
        echo "Creating and running new kubelite-redis container with port mapping (-p 6379:6379 -p 8080:8080)..."
        docker rm -f kubelite-redis 2>/dev/null || true
        docker run -d --name kubelite-redis -p 6379:6379 -p 8080:8080 redis:alpine
    else
        echo "kubelite-redis is already running with port mapping."
    fi
fi

echo "[2/4] Building Worker Docker image (kubelite-worker:latest)..."
cd "$ROOT_DIR/Worker"
docker build -t kubelite-worker:latest .

echo "[3/4] Starting Master Node on :8080..."
cd "$ROOT_DIR/Master"
if [ "$MASTER_MODE" = "host" ]; then
    go run . &
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
        golang:alpine go run . &
    MASTER_PID=$!
fi

echo "Waiting for Master metrics endpoint..."
MASTER_READY=0
for _ in $(seq 1 15); do
    if curl -fsS --max-time 3 http://127.0.0.1:8080/metrics >/dev/null 2>&1; then
        MASTER_READY=1
        break
    fi
    sleep 2
done

if [ "$MASTER_READY" -ne 1 ]; then
    echo "Master API did not become ready at http://127.0.0.1:8080/metrics."
    echo "Check whether another process or container is using port 8080:"
    echo "  docker ps --format '{{.Names}} {{.Ports}}'"
    kill $MASTER_PID 2>/dev/null || true
    docker stop kubelite-master 2>/dev/null || true
    exit 1
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
