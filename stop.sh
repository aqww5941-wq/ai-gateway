#!/usr/bin/env bash
set -e

cd "$(dirname "$0")"

echo "==> Stopping gateway..."

# Kill process on port 8081
PID=$(lsof -ti:8081 2>/dev/null || true)
if [ -n "$PID" ]; then
    kill $PID 2>/dev/null || true
    sleep 0.5
    kill -9 $PID 2>/dev/null || true
    echo "    Killed PID $PID on port 8081"
else
    echo "    No process found on port 8081"
fi

# Also kill vite dev server if running
PID_VITE=$(lsof -ti:5173 2>/dev/null || true)
if [ -n "$PID_VITE" ]; then
    kill $PID_VITE 2>/dev/null || true
    echo "    Killed PID $PID_VITE on port 5173 (vite dev server)"
fi

echo "==> Done."
