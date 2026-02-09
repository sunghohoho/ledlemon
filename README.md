# 🎨 LedLemon

실시간 협업 픽셀 아트 보드 - Reddit r/place 스타일의 멀티플레이어 그리기 애플리케이션

![Ocean Background](https://images.unsplash.com/photo-1505142468610-359e7d316be0?q=80&w=800)

## 📖 소개

LedLemon은 여러 사용자가 동시에 접속하여 50x50 픽셀 캔버스에 실시간으로 그림을 그릴 수 있는 협업 픽셀 아트 애플리케이션입니다. WebSocket 기반 실시간 통신과 AWS 서비스를 활용한 확장 가능한 아키텍처로 구현되었습니다.

### 주요 기능

- 🎨 **실시간 협업 그리기** - 드래그로 연속 픽셀 색칠, 9가지 색상 팔레트
- 👥 **접속자 목록** - 현재 온라인 사용자 실시간 표시
- 💬 **실시간 채팅** - 멀티유저 채팅 시스템 (메시지 영구 저장)
- � **실캔버스 초토화** - 전체 캔버스 초기화 + 시스템 알림
- 💣 **채팅 초토화** - 채팅 히스토리 삭제 + 시스템 알림
- 🌊 **아름다운 UI** - 바다 배경, 그리드 라인, 그라데이션 디자인
- 💾 **데이터 영속성** - 새로고침해도 그림과 채팅 유지

## 🏗️ 아키텍처

### 기술 스택

**Frontend**
- Next.js 16 (React 19)
- TypeScript
- Tailwind CSS
- WebSocket Client

**Backend**
- **Gateway**: Go (Gin Framework) - WebSocket 서버 & 실시간 브로드캐스팅
- **Worker**: Python - 비동기 메시지 처리 & 데이터 저장

**Infrastructure**
- **AWS SQS** - 메시지 큐 (픽셀 업데이트 버퍼링)
- **AWS DynamoDB** - NoSQL 데이터베이스 (픽셀 & 채팅 저장)
- **Redis** - Pub/Sub 메시징 (실시간 브로드캐스트)
- **Docker & Kubernetes (EKS)** - 컨테이너 오케스트레이션
- **Helm Charts** - Kubernetes 패키지 관리
- **ArgoCD** - GitOps 기반 배포

### 시스템 아키텍처

```
┌─────────────────────────────────────────────────────────────────┐
│                         Web Clients                              │
│                    (Next.js + WebSocket)                         │
└────────────────────────┬────────────────────────────────────────┘
                         │ WebSocket (실시간 양방향 통신)
                         ↓
┌─────────────────────────────────────────────────────────────────┐
│                    Gateway (Go + Gin)                            │
│  • WebSocket 연결 관리                                            │
│  • 실시간 브로드캐스팅                                             │
│  • 픽셀 데이터 → SQS 전송                                         │
│  • Redis Pub/Sub 구독                                            │
│  • 채팅 메시지 저장 & 브로드캐스트                                  │
└──────────┬──────────────────────────────┬───────────────────────┘
           │                              │
           ↓                              ↓
    ┌──────────┐                   ┌──────────┐
    │   SQS    │                   │  Redis   │
    │  Queue   │                   │ Pub/Sub  │
    └─────┬────┘                   └────┬─────┘
          │                             │
          ↓                             ↑
    ┌──────────────────────────────────┴─────┐
    │         Worker (Python)                │
    │  • SQS 메시지 폴링 (Long Polling)       │
    │  • DynamoDB에 픽셀 저장                 │
    │  • Redis로 업데이트 발행                │
    └──────────────┬─────────────────────────┘
                   ↓
            ┌──────────────┐
            │  DynamoDB    │
            │  • PixelBoard (픽셀 데이터)      │
            │  • ChatMessages (채팅 메시지)    │
            └──────────────┘
```

## 🔄 데이터 플로우

### 1. 픽셀 그리기 플로우

```
사용자 클릭/드래그
    ↓
① 클라이언트: Optimistic UI 업데이트 (즉시 화면에 표시)
    ↓
② WebSocket → Gateway: 픽셀 데이터 전송
    {type: "pixel", payload: {x, y, color, userId}}
    ↓
③ Gateway → SQS: 메시지 큐에 저장 (비동기 처리)
    ↓
④ Worker: SQS에서 메시지 폴링 (Long Polling, 20초)
    ↓
⑤ Worker → DynamoDB: 픽셀 데이터 저장
    Key: "x:y" (coordinate)
    Attributes: {color, updatedBy, timestamp}
    ↓
⑥ Worker → Redis Pub/Sub: 업데이트 발행
    Channel: "pixel-updates"
    ↓
⑦ Gateway: Redis 구독하여 메시지 수신
    ↓
⑧ Gateway → 모든 클라이언트: WebSocket으로 브로드캐스트
    ↓
⑨ 모든 사용자 화면에 실시간 반영
```

**왜 이렇게 복잡하게?**
- **SQS**: 트래픽 급증 시 버퍼 역할, Worker 장애 시 메시지 보존
- **DynamoDB**: 영구 저장, 새로고침 시 캔버스 복원
- **Redis Pub/Sub**: 빠른 실시간 브로드캐스팅 (밀리초 단위)

### 2. 채팅 시스템 플로우

```
사용자 메시지 입력
    ↓
① WebSocket → Gateway: 채팅 메시지 전송
    {type: "chat", payload: {message}}
    ↓
② Gateway: 메시지 처리
    • userId, timestamp 추가
    • messageId 생성 (timestamp 기반)
    ↓
③ Gateway → DynamoDB: 채팅 메시지 저장 (비동기)
    Table: ChatMessages
    Key: messageId
    Attributes: {userId, message, timestamp, isSystem}
    ↓
④ Gateway → 모든 클라이언트: 즉시 브로드캐스트
    ↓
⑤ 모든 사용자 채팅창에 실시간 표시
```

**새로고침 시:**
```
클라이언트 연결
    ↓
WebSocket 연결 성공
    ↓
Gateway에 채팅 히스토리 요청
    {type: "request_chat"}
    ↓
Gateway → DynamoDB: 모든 채팅 메시지 스캔
    ↓
Gateway → 클라이언트: 히스토리 전송
    {type: "chat_history", messages: [...]}
    ↓
클라이언트: 타임스탬프 순 정렬 후 표시
```

### 3. 멀티유저 상호작용

#### 접속자 관리
```
사용자 A 접속
    ↓
Gateway: 랜덤 닉네임 생성 (예: HappyPanda)
    ↓
Gateway: 접속자 맵에 추가
    clients[websocket] = {Conn, UserID}
    ↓
Gateway → 모든 클라이언트: 접속자 목록 브로드캐스트
    {type: "user_list", users: ["HappyPanda", "SleepyTiger", ...]}
    ↓
모든 사용자 화면에 접속자 목록 업데이트
```

#### 캔버스 초토화
```
사용자 A가 "Clear Canvas" 클릭
    ↓
Gateway → 모든 클라이언트: 초기화 명령
    {type: "clear_canvas"}
    ↓
모든 클라이언트: 캔버스 흰색으로 초기화
    ↓
Gateway → DynamoDB: 시스템 메시지 저장
    ↓
Gateway → 모든 클라이언트: 시스템 알림
    {type: "chat", userId: "System", 
     message: "🔥 HappyPanda님이 캔버스를 초토화 시켰어요!",
     isSystem: true}
```

#### 채팅 초토화
```
사용자 B가 "💣 채팅 초토화" 클릭
    ↓
확인 다이얼로그 표시
    ↓
Gateway → DynamoDB: 모든 채팅 메시지 삭제
    • Scan으로 모든 messageId 조회
    • 각 메시지 DeleteItem 실행
    ↓
Gateway → 모든 클라이언트: 채팅 초기화 명령
    {type: "clear_chat"}
    ↓
모든 클라이언트: 채팅창 비우기
    ↓
Gateway → 모든 클라이언트: 시스템 알림
    "💣 SleepyTiger님이 채팅을 초토화 시켰어요!"
```

## 🔧 AWS 서비스 활용

### Amazon SQS (Simple Queue Service)

**역할**: 픽셀 업데이트 메시지 큐

**사용 이유**:
- **비동기 처리**: Gateway와 Worker 분리, 독립적 확장
- **트래픽 버퍼링**: 순간 트래픽 급증 시 메시지 보존
- **재시도 메커니즘**: Worker 장애 시 메시지 재처리
- **순서 보장 불필요**: 픽셀은 최종 상태만 중요

**설정**:
- Long Polling (20초): 불필요한 API 호출 감소
- Visibility Timeout: 메시지 처리 중 다른 Worker가 중복 처리 방지
- Dead Letter Queue: 실패한 메시지 별도 관리 (선택사항)

**메시지 형식**:
```json
{
  "x": 25,
  "y": 30,
  "color": "#FF0000",
  "userId": "HappyPanda",
  "timestamp": 1707456789123
}
```

### Amazon DynamoDB

**역할**: 픽셀 데이터 & 채팅 메시지 영구 저장

**테이블 1: PixelBoard**
```
Partition Key: coordinate (String) - "x:y" 형식
Attributes:
  - color (String): 색상 코드
  - updatedBy (String): 마지막 수정자
  - timestamp (Number): 업데이트 시간
  - last_updated (Number): Unix timestamp
```

**테이블 2: ChatMessages**
```
Partition Key: messageId (String) - timestamp 기반
Attributes:
  - userId (String): 발신자
  - message (String): 메시지 내용
  - timestamp (Number): 전송 시간
  - isSystem (Boolean): 시스템 메시지 여부
```

**사용 이유**:
- **NoSQL 유연성**: 스키마 변경 용이
- **빠른 읽기/쓰기**: 밀리초 단위 응답
- **자동 확장**: 트래픽에 따라 자동 스케일링
- **Pay-per-request**: 사용한 만큼만 과금

**최적화**:
- Billing Mode: PAY_PER_REQUEST (트래픽 예측 불필요)
- Scan 최소화: 초기 로드 시에만 사용
- Point Query: coordinate로 직접 조회 (Worker)

### Redis (Pub/Sub)

**역할**: 실시간 메시지 브로드캐스팅

**사용 이유**:
- **초저지연**: 밀리초 단위 메시지 전달
- **Pub/Sub 패턴**: 1:N 브로드캐스팅에 최적
- **메모리 기반**: 디스크 I/O 없이 빠른 처리
- **간단한 구조**: 복잡한 설정 불필요

**채널**:
- `pixel-updates`: 픽셀 업데이트 브로드캐스트

**플로우**:
```
Worker → Redis PUBLISH
    ↓
Gateway (Subscriber) → 메시지 수신
    ↓
Gateway → 모든 WebSocket 클라이언트
```

**프로덕션 환경**:
- AWS ElastiCache for Redis 사용 권장
- 클러스터 모드: 고가용성 & 자동 장애 조치
- 백업: 선택사항 (휘발성 데이터)

## 📁 프로젝트 구조

```
.
├── apps/
│   ├── gateway-go/          # Go WebSocket 서버
│   │   ├── main.go          # Gateway 로직
│   │   ├── Dockerfile       # 컨테이너 이미지
│   │   └── go.mod           # Go 의존성
│   ├── worker-python/       # Python 메시지 워커
│   │   ├── worker.py        # Worker 로직
│   │   ├── Dockerfile       # 컨테이너 이미지
│   │   └── requirements.txt # Python 의존성
│   └── web-client/          # Next.js 프론트엔드
│       ├── src/
│       │   └── app/
│       │       └── components/
│       │           └── PixelCanvas.tsx  # 메인 UI 컴포넌트
│       ├── Dockerfile       # 컨테이너 이미지
│       └── package.json     # Node.js 의존성
├── infra/
│   ├── charts/ledlemon/     # Helm 차트
│   │   ├── values.yaml      # 배포 설정
│   │   └── templates/       # Kubernetes 매니페스트
│   ├── argocd/              # ArgoCD 설정
│   │   └── application.yaml # GitOps 배포 정의
│   └── localstack/          # 로컬 개발용
│       └── init-aws.sh      # AWS 리소스 초기화
├── docker-compose.yml       # 로컬 인프라 (Redis, LocalStack)
├── Makefile                 # 개발 명령어
├── dev.sh                   # 통합 실행 스크립트
└── build-and-push.sh        # ECR 배포 스크립트
```

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

# 2. 인프라 시작 (Redis, LocalStack)
docker-compose up -d

# 3. DynamoDB 테이블 생성
aws --endpoint-url=http://localhost:4566 dynamodb create-table \
  --table-name PixelBoard \
  --attribute-definitions AttributeName=coordinate,AttributeType=S \
  --key-schema AttributeName=coordinate,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region ap-northeast-2

aws --endpoint-url=http://localhost:4566 dynamodb create-table \
  --table-name ChatMessages \
  --attribute-definitions AttributeName=messageId,AttributeType=S \
  --key-schema AttributeName=messageId,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region ap-northeast-2

# 4. 전체 실행 (Gateway + Worker + Web)
./dev.sh
```

### 개별 실행

```bash
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

aws dynamodb create-table \
  --table-name ChatMessages \
  --attribute-definitions AttributeName=messageId,AttributeType=S \
  --key-schema AttributeName=messageId,KeyType=HASH \
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

# Helm values 업데이트 (ECR 이미지 경로)
# infra/charts/ledlemon/values.yaml 수정

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

## 🔧 환경 변수

### Gateway (Go)

```bash
AWS_ENDPOINT=http://localhost:4566  # LocalStack (로컬 개발용)
AWS_REGION=ap-northeast-2
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
SQS_QUEUE_URL=https://sqs.ap-northeast-2.amazonaws.com/xxx/pixel-queue
DYNAMODB_TABLE_NAME=PixelBoard
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

### Web Client

```bash
NEXT_PUBLIC_GATEWAY_URL=ws://localhost:8080  # 로컬
# NEXT_PUBLIC_GATEWAY_URL=wss://your-domain.com  # 프로덕션
```

## 🎮 사용 방법

1. 브라우저에서 애플리케이션 접속
2. 자동으로 랜덤 닉네임 생성 (예: HappyPanda)
3. Color Palette에서 원하는 색상 선택
4. 캔버스를 클릭하거나 드래그하여 그리기
5. 좌측 상단에서 현재 접속자 확인
6. 우측 채팅창에서 다른 사용자와 대화
7. "🗑️ Clear Canvas" 버튼으로 캔버스 초기화
8. "💣 채팅 초토화" 버튼으로 채팅 히스토리 삭제

## 🛠️ 개발

### 코드 수정 후 재시작

- **Web Client**: 자동 Hot Reload (재시작 불필요)
- **Gateway**: `Ctrl+C` 후 `make run-gateway`
- **Worker**: `Ctrl+C` 후 `make run-worker`

### 의존성 추가

```bash
# Go
cd apps/gateway-go && go get <package> && go mod tidy

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

# SQS 큐 모니터링
aws sqs get-queue-attributes \
  --queue-url <queue-url> \
  --attribute-names ApproximateNumberOfMessages \
  --region ap-northeast-2

# DynamoDB 테이블 통계
aws dynamodb describe-table \
  --table-name PixelBoard \
  --region ap-northeast-2
```

## 🔍 트러블슈팅

### LocalStack 재시작 시 데이터 손실

**문제**: `docker-compose down` 후 DynamoDB 테이블 사라짐

**해결**:
```bash
# LocalStack 재시작 후 테이블 재생성
aws --endpoint-url=http://localhost:4566 dynamodb create-table ...
```

### Gateway 연결 실패

**문제**: "WebSocket connection failed"

**확인사항**:
1. Gateway가 실행 중인지 확인
2. 포트 8080이 사용 가능한지 확인
3. 환경변수 `NEXT_PUBLIC_GATEWAY_URL` 확인

### Worker SQS 에러

**문제**: "Queue does not exist"

**해결**:
```bash
# SQS 큐 생성
aws --endpoint-url=http://localhost:4566 sqs create-queue \
  --queue-name pixel-queue \
  --region ap-northeast-2
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
