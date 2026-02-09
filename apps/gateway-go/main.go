package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

// Config
var (
	sqsQueueURL      = os.Getenv("SQS_QUEUE_URL")      // e.g., http://localhost:4566/000000000000/pixel-queue
	redisAddr        = os.Getenv("REDIS_ADDR")         // e.g., localhost:6379
	dynamodbEndpoint = os.Getenv("AWS_ENDPOINT")       // For LocalStack
	dynamodbTable    = os.Getenv("DYNAMODB_TABLE_NAME") // e.g., PixelBoard
	chatTable        = "ChatMessages"                   // Chat messages table
)

// Types
type PixelData struct {
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Color     string `json:"color"`
	UserID    string `json:"userId"`
	Timestamp int64  `json:"timestamp"`
}

type ChatMessage struct {
	Type      string `json:"type"`
	UserID    string `json:"userId"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
	IsSystem  bool   `json:"isSystem,omitempty"`
}

type ClearCanvasMessage struct {
	Type   string `json:"type"`
	UserID string `json:"userId"`
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

type IncomingMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Global Clients
var (
	sqsClient      *sqs.Client
	dynamodbClient *dynamodb.Client
	redisClient    *redis.Client
	upgrader       = websocket.Upgrader{
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

	// Setup DynamoDB client
	dynamodbClient = dynamodb.NewFromConfig(cfg)
	if dynamodbTable == "" {
		dynamodbTable = "PixelBoard"
	}

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
		var incoming IncomingMessage
		err := ws.ReadJSON(&incoming)
		if err != nil {
			log.Printf("Read error: %v", err)
			break
		}

		switch incoming.Type {
		case "pixel":
			var pixel PixelData
			if err := json.Unmarshal(incoming.Payload, &pixel); err != nil {
				log.Printf("Failed to parse pixel data: %v", err)
				continue
			}
			pixel.Timestamp = time.Now().UnixMilli()
			go sendToSQS(pixel)

		case "chat":
			var chat ChatMessage
			if err := json.Unmarshal(incoming.Payload, &chat); err != nil {
				log.Printf("Failed to parse chat message: %v", err)
				continue
			}
			chat.Type = "chat"
			chat.UserID = userID
			chat.Timestamp = time.Now().UnixMilli()
			
			// Save to DynamoDB
			go saveChatMessage(chat)
			
			// Broadcast to all clients
			broadcastJSON(chat)

		case "clear_canvas":
			// Clear all pixels from DynamoDB
			go clearAllPixels()
			
			// Broadcast clear canvas to all clients
			clearMsg := map[string]interface{}{
				"type": "clear_canvas",
			}
			broadcastJSON(clearMsg)

			// Send system message to chat
			systemMsg := ChatMessage{
				Type:      "chat",
				UserID:    "System",
				Message:   fmt.Sprintf("🔥 %s님이 캔버스를 초토화 시켰어요!", userID),
				Timestamp: time.Now().UnixMilli(),
				IsSystem:  true,
			}
			go saveChatMessage(systemMsg)
			broadcastJSON(systemMsg)

		case "clear_chat":
			// Clear all chat messages from DynamoDB
			go clearAllChatMessages()
			
			// Broadcast clear chat to all clients
			clearChatMsg := map[string]interface{}{
				"type": "clear_chat",
			}
			broadcastJSON(clearChatMsg)
			
			// Send system message
			systemMsg := ChatMessage{
				Type:      "chat",
				UserID:    "System",
				Message:   fmt.Sprintf("💣 %s님이 채팅을 초토화 시켰어요!", userID),
				Timestamp: time.Now().UnixMilli(),
				IsSystem:  true,
			}
			go saveChatMessage(systemMsg)
			broadcastJSON(systemMsg)

		case "request_canvas":
			// Load canvas state from DynamoDB and send to client
			go sendCanvasState(ws)
			
		case "request_chat":
			// Load chat history from DynamoDB and send to client
			go sendChatHistory(ws)
		}
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

func sendCanvasState(ws *websocket.Conn) {
	ctx := context.TODO()
	
	// Scan DynamoDB for all pixels
	result, err := dynamodbClient.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(dynamodbTable),
	})
	
	if err != nil {
		log.Printf("Failed to scan DynamoDB: %v", err)
		return
	}

	pixels := []PixelData{}
	for _, item := range result.Items {
		var dbPixel struct {
			Coordinate string `dynamodbav:"coordinate"`
			Color      string `dynamodbav:"color"`
		}
		
		if err := attributevalue.UnmarshalMap(item, &dbPixel); err != nil {
			log.Printf("Failed to unmarshal item: %v", err)
			continue
		}

		// Parse coordinate "x:y"
		var x, y int
		fmt.Sscanf(dbPixel.Coordinate, "%d:%d", &x, &y)
		
		pixels = append(pixels, PixelData{
			X:     x,
			Y:     y,
			Color: dbPixel.Color,
		})
	}

	// Send canvas state to client
	response := map[string]interface{}{
		"type":   "canvas_state",
		"pixels": pixels,
	}
	
	data, _ := json.Marshal(response)
	if err := ws.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("Failed to send canvas state: %v", err)
	}
}

func saveChatMessage(msg ChatMessage) {
	ctx := context.TODO()
	
	messageID := fmt.Sprintf("%d", msg.Timestamp)
	
	item := map[string]interface{}{
		"messageId": messageID,
		"userId":    msg.UserID,
		"message":   msg.Message,
		"timestamp": msg.Timestamp,
		"isSystem":  msg.IsSystem,
	}
	
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		log.Printf("Failed to marshal chat message: %v", err)
		return
	}
	
	_, err = dynamodbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(chatTable),
		Item:      av,
	})
	
	if err != nil {
		log.Printf("Failed to save chat message: %v", err)
	}
}

func sendChatHistory(ws *websocket.Conn) {
	ctx := context.TODO()
	
	result, err := dynamodbClient.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(chatTable),
	})
	
	if err != nil {
		log.Printf("Failed to scan chat messages: %v", err)
		return
	}

	messages := []ChatMessage{}
	for _, item := range result.Items {
		var dbMsg struct {
			MessageID string `dynamodbav:"messageId"`
			UserID    string `dynamodbav:"userId"`
			Message   string `dynamodbav:"message"`
			Timestamp int64  `dynamodbav:"timestamp"`
			IsSystem  bool   `dynamodbav:"isSystem"`
		}
		
		if err := attributevalue.UnmarshalMap(item, &dbMsg); err != nil {
			log.Printf("Failed to unmarshal chat message: %v", err)
			continue
		}
		
		messages = append(messages, ChatMessage{
			Type:      "chat",
			UserID:    dbMsg.UserID,
			Message:   dbMsg.Message,
			Timestamp: dbMsg.Timestamp,
			IsSystem:  dbMsg.IsSystem,
		})
	}

	// Send chat history to client
	response := map[string]interface{}{
		"type":     "chat_history",
		"messages": messages,
	}
	
	data, _ := json.Marshal(response)
	if err := ws.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("Failed to send chat history: %v", err)
	}
}

func clearAllChatMessages() {
	ctx := context.TODO()
	
	// Scan all messages
	result, err := dynamodbClient.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(chatTable),
	})
	
	if err != nil {
		log.Printf("Failed to scan chat messages for deletion: %v", err)
		return
	}

	// Delete each message
	for _, item := range result.Items {
		var dbMsg struct {
			MessageID string `dynamodbav:"messageId"`
		}
		
		if err := attributevalue.UnmarshalMap(item, &dbMsg); err != nil {
			continue
		}
		
		key, _ := attributevalue.MarshalMap(map[string]string{
			"messageId": dbMsg.MessageID,
		})
		
		_, err = dynamodbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(chatTable),
			Key:       key,
		})
		
		if err != nil {
			log.Printf("Failed to delete chat message: %v", err)
		}
	}
}

func clearAllPixels() {
	ctx := context.TODO()
	
	// Scan all pixels
	result, err := dynamodbClient.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(dynamodbTable),
	})
	
	if err != nil {
		log.Printf("Failed to scan pixels for deletion: %v", err)
		return
	}

	// Delete each pixel
	for _, item := range result.Items {
		var dbPixel struct {
			Coordinate string `dynamodbav:"coordinate"`
		}
		
		if err := attributevalue.UnmarshalMap(item, &dbPixel); err != nil {
			continue
		}
		
		key, _ := attributevalue.MarshalMap(map[string]string{
			"coordinate": dbPixel.Coordinate,
		})
		
		_, err = dynamodbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(dynamodbTable),
			Key:       key,
		})
		
		if err != nil {
			log.Printf("Failed to delete pixel: %v", err)
		}
	}
	
	log.Printf("Cleared all pixels from DynamoDB")
}
