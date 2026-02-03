# 🎨 LedLemon

실시간 협업 픽셀 아트 보드 - Reddit r/place 스타일의 멀티플레이어 그리기 애플리케이션

![Ocean Background](https://images.unsplash.com/photo-1505142468610-359e7d316be0?q=80&w=800)

## 📖 소개

LedLemon은 여러 사용자가 동시에 접속하여 50x50 픽셀 캔버스에 실시간으로 그림을 그릴 수 있는 협업 픽셀 아트 애플리케이션입니다.

### 주요 기능

- 🎨 **실시간 협업 그리기** - 드래그로 연속 픽셀 색칠
- 👥 **접속자 목록** - 현재 온라인 사용자 실시간 표시
- 🌊 **아름다운 UI** - 바다 배경과 그리드 라인
- 🔄 **실시간 동기화** - WebSocket 기반 즉각적인 업데이트
- 🎭 **랜덤 닉네임** - 자동 생성되는 재미있는 사용자 이름

## 🏗️ 아키텍처

### 기술 스택

**Frontend**
- Next.js 16 (React 19)
- TypeScript
- Tailwind CSS
- WebSocket Client

**Backend**
- **Gateway**: Go (Gin Framework) - WebSocket 서버
- **Worker**: Python - 메시지 처리 및 데이터 저장

**Infrastructure**
- Redis - Pub/Sub 메시징
- AWS SQS - 메시지 큐
- AWS DynamoDB - 픽셀 데이터 저장
- Docker & Kubernetes (EKS)
- Helm Charts
- ArgoCD (GitOps)

### 시스템 구조

```
┌─────────────┐
│  Web Client │ (Next.js)
└──────┬──────┘
       │ WebSocket
       ↓
┌─────────────┐      ┌─────────┐
│   Gateway   │─────→│   SQS   │
│    (Go)     │      └────┬────┘
└──────┬──────┘           │
       │                  ↓
       │            ┌──────────┐      ┌──────────┐
       │            │  Worker  │─────→│ DynamoDB │
       │            │ (Python) │      └──────────┘
       │            └────┬─────┘
       │                 │
       │                 ↓
       │            ┌─────────┐
       └────────────│  Redis  │
         Subscribe  │ Pub/Sub │
                    └─────────┘
```

### 데이터 플로우

1. 사용자가 픽셀 클릭/드래그
2. WebSocket으로 Gateway에 전송
3. Gateway가 SQS에 메시지 발행
4. Worker가 SQS에서 메시지 수신
5. Worker가 DynamoDB에 픽셀 데이터 저장
6. Worker가 Redis Pub/Sub으로 업데이트 발행
7. Gateway가 Redis 구독하여 모든 클라이언트에게 브로드캐스트

## 🚀 로컬 실행

### 사전 요구사항

- Docker & Docker Compose
- Go 1.23+
- Node.js 20+
- Python 3.11+
- AWS CLI (선택사항)

### 빠른 시작

```bash
# 1. 의존성 설치
make setup

# 2. 전체 실행 (인프라 + 모든 서비스)
./dev.sh
```

### 개별 실행

```bash
# 인프라 시작 (Redis, LocalStack)
make run-infra

# Gateway 실행
make run-gateway

# Worker 실행
make run-worker

# Web Client 실행
make run-web
```

### 접속

- **Web Client**: http://localhost:3000
- **Gateway WebSocket**: ws://localhost:8080/ws

## 🐳 Docker 빌드

각 서비스별 Dockerfile이 준비되어 있습니다:

```bash
# Gateway
docker build -t ledlemon-gateway apps/gateway-go/

# Worker
docker build -t ledlemon-worker apps/worker-python/

# Web Client
docker build -t ledlemon-web apps/web-client/
```

## ☸️ Kubernetes 배포

### ECR에 이미지 푸시

```bash
# 모든 이미지 빌드 및 ECR 푸시
./build-and-push.sh
```

### AWS 리소스 생성

```bash
# DynamoDB 테이블
aws dynamodb create-table \
  --table-name PixelBoard \
  --attribute-definitions AttributeName=coordinate,AttributeType=S \
  --key-schema AttributeName=coordinate,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region ap-northeast-2

# SQS 큐
aws sqs create-queue \
  --queue-name pixel-queue \
  --region ap-northeast-2
```

### Helm 배포

```bash
# EKS 클러스터 연결
aws eks update-kubeconfig --region ap-northeast-2 --name <cluster-name>

# Helm으로 배포
helm install ledlemon infra/charts/ledlemon -n pixel-game --create-namespace

# 상태 확인
kubectl get pods -n pixel-game
kubectl get svc -n pixel-game
```

### ArgoCD 배포

```bash
kubectl apply -f infra/argocd/application.yaml
```

## 📁 프로젝트 구조

```
.
├── apps/
│   ├── gateway-go/          # Go WebSocket 서버
│   │   ├── main.go
│   │   ├── Dockerfile
│   │   └── go.mod
│   ├── worker-python/       # Python 메시지 워커
│   │   ├── worker.py
│   │   ├── Dockerfile
│   │   └── requirements.txt
│   └── web-client/          # Next.js 프론트엔드
│       ├── src/
│       ├── Dockerfile
│       └── package.json
├── infra/
│   ├── charts/ledlemon/     # Helm 차트
│   │   ├── values.yaml
│   │   └── templates/
│   ├── argocd/              # ArgoCD 설정
│   │   └── application.yaml
│   └── localstack/          # 로컬 개발용
│       └── init-aws.sh
├── docker-compose.yml       # 로컬 인프라
├── Makefile                 # 개발 명령어
├── dev.sh                   # 통합 실행 스크립트
└── build-and-push.sh        # ECR 배포 스크립트
```

## 🔧 환경 변수

### Gateway (Go)

```bash
AWS_ENDPOINT=http://localhost:4566  # LocalStack (로컬 개발용)
AWS_REGION=ap-northeast-2
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
SQS_QUEUE_URL=https://sqs.ap-northeast-2.amazonaws.com/xxx/pixel-queue
REDIS_ADDR=localhost:6379
```

### Worker (Python)

```bash
AWS_ENDPOINT=http://localhost:4566  # LocalStack (로컬 개발용)
AWS_REGION=ap-northeast-2
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
SQS_QUEUE_URL=https://sqs.ap-northeast-2.amazonaws.com/xxx/pixel-queue
REDIS_ADDR=localhost
REDIS_PORT=6379
DYNAMODB_TABLE_NAME=PixelBoard
```

## 🎮 사용 방법

1. 브라우저에서 애플리케이션 접속
2. 상단의 색상 버튼을 클릭하여 원하는 색상 선택
3. 캔버스를 클릭하거나 드래그하여 그리기
4. 우측 상단에서 현재 접속자 확인
5. "초기화" 버튼으로 본인 화면 리셋 (다른 사용자에게 영향 없음)

## 🛠️ 개발

### 코드 수정 후 재시작

- **Web Client**: 자동 Hot Reload (재시작 불필요)
- **Gateway**: `Ctrl+C` 후 `make run-gateway`
- **Worker**: `Ctrl+C` 후 `make run-worker`

### 의존성 추가

```bash
# Go
cd apps/gateway-go && go get <package>

# Python
cd apps/worker-python && ./venv/bin/pip install <package>
echo "<package>" >> requirements.txt

# Node.js
cd apps/web-client && npm install <package>
```

## 📊 모니터링

```bash
# Pod 로그 확인
kubectl logs -f deployment/ledlemon-gateway -n pixel-game
kubectl logs -f deployment/ledlemon-worker -n pixel-game
kubectl logs -f deployment/ledlemon-web -n pixel-game

# 리소스 사용량
kubectl top pods -n pixel-game
```

## 🤝 기여

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 라이선스

MIT License

## 🙏 감사의 말

- Reddit r/place에서 영감을 받았습니다
- Unsplash의 아름다운 바다 사진 제공

---

Made with ❤️ by LedLemon Team
