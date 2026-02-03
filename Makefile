.PHONY: setup run-all

setup:
	@echo "Setting up dependencies..."
	cd apps/web-client && npm install
	cd apps/gateway-go && go mod tidy
	cd apps/worker-python && python3 -m venv venv && ./venv/bin/pip install -r requirements.txt

run-infra:
	docker-compose up -d redis localstack

run-gateway:
	cd apps/gateway-go && \
	AWS_ENDPOINT=http://localhost:4566 \
	AWS_REGION=ap-northeast-2 \
	AWS_ACCESS_KEY_ID=test \
	AWS_SECRET_ACCESS_KEY=test \
	SQS_QUEUE_URL=http://localhost:4566/000000000000/pixel-queue \
	REDIS_ADDR=localhost:6379 \
	go run main.go

run-worker:
	cd apps/worker-python && \
	AWS_ENDPOINT=http://localhost:4566 \
	AWS_REGION=ap-northeast-2 \
	AWS_ACCESS_KEY_ID=test \
	AWS_SECRET_ACCESS_KEY=test \
	SQS_QUEUE_URL=http://localhost:4566/000000000000/pixel-queue \
	REDIS_ADDR=localhost \
	./venv/bin/python3 worker.py

run-web:
	cd apps/web-client && npm run dev
