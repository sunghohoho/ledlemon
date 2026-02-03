package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

// Config
var (
	sqsQueueURL = os.Getenv("SQS_QUEUE_URL") // e.g., http://localhost:4566/000000000000/pixel-queue
	redisAddr   = os.Getenv("REDIS_ADDR")    // e.g., localhost:6379
)

// Types
type PixelData struct {
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Color     string `json:"color"`
	UserID    string `json:"userId"`
	Timestamp int64  `json:"timestamp"`
}

type UserJoinMessage struct {
	Type   string   `json:"type"`
	UserID string   `json:"userId"`
	Users  []string `json:"users"`
}

type UserLeaveMessage struct {
	Type   string `json:"type"`
	UserID string `json:"userId"`
}

type ClientInfo struct {
	Conn   *websocket.Conn
	UserID string
}

// Global Clients
var (
	sqsClient   *sqs.Client
	redisClient *redis.Client
	upgrader    = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	clients   = make(map[*websocket.Conn]*ClientInfo)
	clientsMu sync.Mutex
)

func main() {
	// 1. Setup AWS SQS
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}
	// LocalStack endpoint override if needed
	if os.Getenv("AWS_ENDPOINT") != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				PartitionID:   "aws",
				URL:           os.Getenv("AWS_ENDPOINT"),
				SigningRegion: "ap-northeast-2",
			}, nil
		})
		cfg, err = config.LoadDefaultConfig(context.TODO(), config.WithEndpointResolverWithOptions(customResolver))
	}
	sqsClient = sqs.NewFromConfig(cfg)

	// Ensure SQS queue exists
	ensureSQSQueue()

	// 2. Setup Redis
	redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// 3. Start Redis Subscriber (Broadcast loop)
	go startRedisSubscriber()

	// 4. Setup Router
	r := gin.Default()
	r.GET("/ws", handleWebSocket)

	log.Println("Gateway running on :8080")
	r.Run(":8080")
}

func handleWebSocket(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer ws.Close()

	// Generate user ID from query param or create new one
	userID := c.Query("userId")
	if userID == "" {
		userID = "Anonymous"
	}

	// Register client
	clientInfo := &ClientInfo{
		Conn:   ws,
		UserID: userID,
	}
	clientsMu.Lock()
	clients[ws] = clientInfo
	clientsMu.Unlock()

	// Broadcast user join
	broadcastUserList()

	defer func() {
		clientsMu.Lock()
		delete(clients, ws)
		clientsMu.Unlock()
		
		// Broadcast user leave
		leaveMsg := UserLeaveMessage{
			Type:   "user_leave",
			UserID: userID,
		}
		broadcastJSON(leaveMsg)
	}()

	for {
		var pixel PixelData
		err := ws.ReadJSON(&pixel)
		if err != nil {
			log.Printf("Read error: %v", err)
			break
		}
		
		pixel.Timestamp = time.Now().UnixMilli()
		
		// Send to SQS
		go sendToSQS(pixel)
	}
}

func sendToSQS(data PixelData) {
	body, _ := json.Marshal(data)
	_, err := sqsClient.SendMessage(context.TODO(), &sqs.SendMessageInput{
		MessageBody: aws.String(string(body)),
		QueueUrl:    aws.String(sqsQueueURL),
	})
	if err != nil {
		log.Printf("Failed to send to SQS: %v", err)
	}
}

func startRedisSubscriber() {
	ctx := context.Background()
	pubsub := redisClient.Subscribe(ctx, "pixel-updates")
	defer pubsub.Close()

	ch := pubsub.Channel()

	for msg := range ch {
		// Broadcast to all clients
		broadcast([]byte(msg.Payload))
	}
}

func broadcast(message []byte) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	for client := range clients {
		err := client.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("Write error: %v", err)
			client.Close()
			delete(clients, client)
		}
	}
}

func broadcastJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("Marshal error: %v", err)
		return
	}
	broadcast(data)
}

func broadcastUserList() {
	clientsMu.Lock()
	userList := make([]string, 0, len(clients))
	for _, info := range clients {
		userList = append(userList, info.UserID)
	}
	clientsMu.Unlock()

	msg := UserJoinMessage{
		Type:  "user_list",
		Users: userList,
	}
	broadcastJSON(msg)
}

func ensureSQSQueue() {
	queueName := "pixel-queue"
	
	// Try to get queue URL
	_, err := sqsClient.GetQueueUrl(context.TODO(), &sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	})
	
	if err != nil {
		// Queue doesn't exist, create it
		log.Printf("Creating SQS queue: %s", queueName)
		_, err := sqsClient.CreateQueue(context.TODO(), &sqs.CreateQueueInput{
			QueueName: aws.String(queueName),
		})
		if err != nil {
			log.Printf("Warning: Failed to create SQS queue: %v", err)
		} else {
			log.Printf("SQS queue created successfully")
		}
	} else {
		log.Printf("SQS queue already exists")
	}
}
