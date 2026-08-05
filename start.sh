#!/usr/bin/env bash
set -e

cd "$(dirname "$0")"

# Kill any existing process on :8081
OLD_PID=$(lsof -ti:8081 2>/dev/null || true)
if [ -n "$OLD_PID" ]; then
    echo "==> Killing existing process on :8081 (PID $OLD_PID)..."
    kill $OLD_PID 2>/dev/null || true
    sleep 1
    kill -9 $OLD_PID 2>/dev/null || true
    echo "==> Old process cleared."
fi

echo "==> Building frontend..."
cd web
npm ci --silent
npm run build
cd ..

rm -rf internal/static/dist
cp -r web/dist internal/static/dist

echo "==> Building backend..."
go build -o bin/gateway ./cmd/gateway

echo "==> Starting gateway on :8081 ..."
./bin/gateway &
PID=$!
echo "    PID: $PID"
echo "    Admin: http://localhost:8081/admin/dashboard/"

# Poll until our PID is listening on :8081
for i in $(seq 1 10); do
    sleep 0.5
    if ! kill -0 $PID 2>/dev/null; then
        echo "==> Gateway failed to start, check logs above."
        exit 1
    fi
    LISTEN_PID=$(lsof -ti:8081 2>/dev/null || true)
    if [ "$LISTEN_PID" = "$PID" ]; then
        echo "==> Gateway is running. Use ./stop.sh to stop."
        break
    fi
done

wait $PID
