#!/bin/bash

# Function to kill all background processes on exit
cleanup() {
    echo "Stopping all services..."
    kill $(jobs -p) 2>/dev/null
    docker-compose down
}
trap cleanup EXIT

echo "Starting Infrastructure (Redis, LocalStack)..."
docker-compose up -d redis localstack

echo "Waiting for LocalStack to be ready..."
sleep 8

echo "Creating DynamoDB tables..."
aws --endpoint-url=http://localhost:4566 dynamodb create-table \
    --table-name PixelBoard \
    --attribute-definitions AttributeName=coordinate,AttributeType=S \
    --key-schema AttributeName=coordinate,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST \
    --region ap-northeast-2 2>/dev/null && echo "✓ PixelBoard table created" || echo "✓ PixelBoard table already exists"

aws --endpoint-url=http://localhost:4566 dynamodb create-table \
    --table-name ChatMessages \
    --attribute-definitions AttributeName=messageId,AttributeType=S \
    --key-schema AttributeName=messageId,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST \
    --region ap-northeast-2 2>/dev/null && echo "✓ ChatMessages table created" || echo "✓ ChatMessages table already exists"

echo ""
echo "Starting Gateway..."
(make run-gateway) &

echo "Starting Worker..."
(make run-worker) &

echo "Starting Web Client..."
(make run-web) &

echo ""
echo "=========================================="
echo "🎨 LedLemon is running!"
echo "=========================================="
echo "Web Client: http://localhost:3000"
echo "Gateway: ws://localhost:8080/ws"
echo ""
echo "Press Ctrl+C to stop all services"
echo "=========================================="

# Keep script running to maintain background processes
wait
