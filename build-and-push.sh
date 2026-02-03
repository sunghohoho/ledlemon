#!/bin/bash

# AWS ECR 설정
AWS_REGION="ap-northeast-2"
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
ECR_REGISTRY="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"

# ECR 로그인
aws ecr get-login-password --region ${AWS_REGION} | docker login --username AWS --password-stdin ${ECR_REGISTRY}

# ECR 리포지토리 생성 (없으면)
aws ecr create-repository --repository-name pixel-game/gateway --region ${AWS_REGION} 2>/dev/null || true
aws ecr create-repository --repository-name pixel-game/worker --region ${AWS_REGION} 2>/dev/null || true
aws ecr create-repository --repository-name pixel-game/web --region ${AWS_REGION} 2>/dev/null || true

# 이미지 빌드 및 푸시
echo "Building and pushing Gateway..."
docker build -t ${ECR_REGISTRY}/pixel-game/gateway:latest apps/gateway-go/
docker push ${ECR_REGISTRY}/pixel-game/gateway:latest

echo "Building and pushing Worker..."
docker build -t ${ECR_REGISTRY}/pixel-game/worker:latest apps/worker-python/
docker push ${ECR_REGISTRY}/pixel-game/worker:latest

echo "Building and pushing Web Client..."
docker build -t ${ECR_REGISTRY}/pixel-game/web:latest apps/web-client/
docker push ${ECR_REGISTRY}/pixel-game/web:latest

echo "All images pushed successfully!"
echo "Update your Helm values with:"
echo "  gateway.image.repository: ${ECR_REGISTRY}/pixel-game/gateway"
echo "  worker.image.repository: ${ECR_REGISTRY}/pixel-game/worker"
echo "  web.image.repository: ${ECR_REGISTRY}/pixel-game/web"
