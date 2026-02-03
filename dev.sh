#!/bin/bash

# Function to kill all background processes on exit
cleanup() {
    echo "Stopping all services..."
    kill $(jobs -p)
    docker-compose down
}
trap cleanup EXIT

echo "Starting Infrastructure (Redis, LocalStack)..."
docker-compose up -d redis localstack
echo "Waiting for LocalStack to be ready..."
sleep 5 # Simple wait, or use a healthcheck loop

echo "Starting Gateway..."
(make run-gateway) &

echo "Starting Worker..."
(make run-worker) &

echo "Starting Web Client..."
(make run-web) &

# Keep script running to maintain background processes
wait
