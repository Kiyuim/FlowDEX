package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"websocket/config"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

type TokenWebSocketServer struct {
	clients      map[*websocket.Conn]*TokenClient
	clientsMu    sync.RWMutex
	klineClients map[*websocket.Conn]*KlineClient
	klineMu      sync.RWMutex
	redisClient  *redis.Client
	upgrader     websocket.Upgrader
	config       config.Config
}

// KlineClient is a WebSocket client subscribed to real-time K-line updates for
// one trading pair. dataflow publishes updates to the "kline:updates" Redis
// channel (see dataflow/internal/mqs/consumers/trade_consumer.go); this server
// fans those out to matching clients. Previously this whole path only existed
// as commented-out dead code (websocket/websocket.go) — see
// docs/项目已知问题与修复记录.md.
type KlineClient struct {
	conn        *websocket.Conn
	pairAddress string
	chainId     int64
	send        chan []byte
}

// KlineUpdateMessage mirrors dataflow's publish payload field-for-field.
type KlineUpdateMessage struct {
	Type string     `json:"type"`
	Data *KlineData `json:"data"`
}

type KlineData struct {
	PairAddress string  `json:"pair_address"`
	ChainId     int64   `json:"chain_id"`
	Interval    string  `json:"interval"`
	CandleTime  int64   `json:"candle_time"`
	Open        float64 `json:"open"`
	High        float64 `json:"high"`
	Low         float64 `json:"low"`
	Close       float64 `json:"close"`
	Volume      float64 `json:"volume"`
	Timestamp   int64   `json:"timestamp"`
}

type TokenClient struct {
	conn       *websocket.Conn
	chainId    int64
	categories []string // new_creation, completing, completed
	send       chan []byte
}

type TokenMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type TokenData struct {
	ChainId       int64   `json:"chain_id"`
	TokenAddress  string  `json:"token_address"`
	PairAddress   string  `json:"pair_address"`
	TokenName     string  `json:"token_name"`
	TokenSymbol   string  `json:"token_symbol"`
	TokenIcon     string  `json:"token_icon"`
	LaunchTime    int64   `json:"launch_time"`
	MktCap        float64 `json:"mkt_cap"`
	HoldCount     int64   `json:"hold_count"`
	Change24      float64 `json:"change_24"`
	Txs24h        int64   `json:"txs_24h"`
	PumpStatus    int     `json:"pump_status"` // 1=new_creation, 2=completing, 4=completed
	OldPumpStatus int     `json:"old_pump_status,omitempty"`
}

type SubscriptionMessage struct {
	Type string `json:"type"`
	Data struct {
		ChainId    int64    `json:"chain_id"`
		Categories []string `json:"categories"`
	} `json:"data"`
}

func NewTokenWebSocketServer(cfg config.Config) *TokenWebSocketServer {
	// 构建 Redis 地址
	redisAddr := fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port)

	return &TokenWebSocketServer{
		clients:      make(map[*websocket.Conn]*TokenClient),
		klineClients: make(map[*websocket.Conn]*KlineClient),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins in development - restrict in production
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		redisClient: redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		}),
		config: cfg,
	}
}

func (tws *TokenWebSocketServer) HandleTokenWebSocket(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	// Handle preflight OPTIONS request
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	conn, err := tws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logx.Errorf("🔌 [TOKEN WS] WebSocket upgrade failed: %v", err)
		return
	}

	// Get query parameters
	chainIdStr := r.URL.Query().Get("chain_id")
	if chainIdStr == "" {
		chainIdStr = "100000" // Default to SOL chain
	}

	chainId, err := strconv.ParseInt(chainIdStr, 10, 64)
	if err != nil {
		logx.Errorf("🔌 [TOKEN WS] Invalid chain_id: %s", chainIdStr)
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error": "Invalid chain_id"}`))
		conn.Close()
		return
	}

	client := &TokenClient{
		conn:       conn,
		chainId:    chainId,
		categories: []string{"new_creation", "completing", "completed"}, // Default categories
		send:       make(chan []byte, 256),
	}

	tws.clientsMu.Lock()
	tws.clients[conn] = client
	tws.clientsMu.Unlock()

	logx.Infof("🎯 [TOKEN WS] New token WebSocket client connected for chain: %d", chainId)

	// Send welcome message
	tws.sendWelcomeMessage(client)

	// Start goroutines for handling client
	go tws.writeLoop(client)
	go tws.readLoop(client)
}

func (tws *TokenWebSocketServer) sendWelcomeMessage(client *TokenClient) {
	welcomeMsg := TokenMessage{
		Type: "connection_established",
		Data: map[string]interface{}{
			"chain_id":   client.chainId,
			"categories": client.categories,
			"status":     "connected",
			"timestamp":  time.Now().Unix(),
		},
	}

	data, _ := json.Marshal(welcomeMsg)
	select {
	case client.send <- data:
		logx.Infof("✅ [TOKEN WS] Welcome message sent to client for chain: %d", client.chainId)
	default:
		logx.Errorf("❌ [TOKEN WS] Failed to send welcome message - channel full")
	}
}

func (tws *TokenWebSocketServer) writeLoop(client *TokenClient) {
	ticker := time.NewTicker(30 * time.Second) // Ping every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Channel was closed
				logx.Infof("📤 [TOKEN WS] Send channel closed for chain: %d", client.chainId)
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				logx.Errorf("📤 [TOKEN WS] Error writing message to client for chain %d: %v", client.chainId, err)
				return
			}
			logx.Infof("📤 [TOKEN WS] Successfully sent message to client for chain: %d", client.chainId)

		case <-ticker.C:
			// Send ping to keep connection alive
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logx.Errorf("📤 [TOKEN WS] Error sending ping to client for chain %d: %v", client.chainId, err)
				return
			}
			logx.Infof("📤 [TOKEN WS] Sent ping to client for chain: %d", client.chainId)
		}
	}
}

func (tws *TokenWebSocketServer) readLoop(client *TokenClient) {
	defer func() {
		tws.removeClient(client.conn)
		client.conn.Close()
	}()

	// Set up pong handler for ping/pong keep-alive
	client.conn.SetPongHandler(func(string) error {
		logx.Infof("📳 [TOKEN WS] Received pong from client for chain: %d", client.chainId)
		return nil
	})

	for {
		messageType, message, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				logx.Errorf("🔌 [TOKEN WS] Unexpected close error for chain %d: %v", client.chainId, err)
			} else {
				logx.Infof("🔌 [TOKEN WS] Client disconnected gracefully for chain: %d (error: %v)", client.chainId, err)
			}
			break
		}

		// Handle subscription messages
		if messageType == websocket.TextMessage {
			var subMsg SubscriptionMessage
			if err := json.Unmarshal(message, &subMsg); err == nil && subMsg.Type == "subscribe" {
				client.categories = subMsg.Data.Categories
				logx.Infof("📨 [TOKEN WS] Client updated subscription for chain %d: %v", client.chainId, client.categories)
			} else {
				logx.Infof("📨 [TOKEN WS] Received message from client for chain %d: %s", client.chainId, string(message))
			}
		}
	}
}

func (tws *TokenWebSocketServer) removeClient(conn *websocket.Conn) {
	tws.clientsMu.Lock()
	defer tws.clientsMu.Unlock()

	if client, exists := tws.clients[conn]; exists {
		close(client.send)
		delete(tws.clients, conn)
		logx.Infof("🛑 [TOKEN WS] Removed client for chain: %d", client.chainId)
	}
}

// BroadcastNewToken sends new token notification to all subscribed clients
func (tws *TokenWebSocketServer) BroadcastNewToken(tokenData *TokenData) {
	message := TokenMessage{
		Type: "new_token",
		Data: tokenData,
	}

	data, err := json.Marshal(message)
	if err != nil {
		logx.Errorf("❌ [TOKEN WS] Error marshaling new token message: %v", err)
		return
	}

	tws.clientsMu.RLock()
	defer tws.clientsMu.RUnlock()

	category := tws.pumpStatusToCategory(tokenData.PumpStatus)

	fmt.Println("data is:", string(data))

	fmt.Println("the length of tws.clients is:", len(tws.clients))

	for conn, client := range tws.clients {
		// Check if client is subscribed to this chain and category
		fmt.Println("client.chainId is:", client.chainId)
		fmt.Println("tokenData.ChainId is:", tokenData.ChainId)
		fmt.Println("category is:", category)
		fmt.Println("tws.isSubscribedToCategory(client, category) is:", tws.isSubscribedToCategory(client, category))
		if client.chainId == tokenData.ChainId && tws.isSubscribedToCategory(client, category) {
			select {
			case client.send <- data:
				logx.Infof("🆕 [TOKEN WS] Sent new token %s to client for chain %d", tokenData.TokenName, client.chainId)
			default:
				logx.Errorf("❌ [TOKEN WS] Failed to send new token - channel full, removing client")
				go func(c *websocket.Conn) {
					tws.removeClient(c)
					c.Close()
				}(conn)
			}
		}
	}
}

// BroadcastTokenStatusUpdate sends token status update to all subscribed clients
func (tws *TokenWebSocketServer) BroadcastTokenStatusUpdate(tokenData *TokenData) {
	message := TokenMessage{
		Type: "token_status_update",
		Data: tokenData,
	}

	data, err := json.Marshal(message)
	if err != nil {
		logx.Errorf("❌ [TOKEN WS] Error marshaling token update message: %v", err)
		return
	}

	tws.clientsMu.RLock()
	defer tws.clientsMu.RUnlock()

	for conn, client := range tws.clients {
		// Check if client is subscribed to this chain
		if client.chainId == tokenData.ChainId {
			select {
			case client.send <- data:
				logx.Infof("🔄 [TOKEN WS] Sent token update %s to client for chain %d", tokenData.TokenName, client.chainId)
			default:
				logx.Errorf("❌ [TOKEN WS] Failed to send token update - channel full, removing client")
				go func(c *websocket.Conn) {
					tws.removeClient(c)
					c.Close()
				}(conn)
			}
		}
	}
}

func (tws *TokenWebSocketServer) pumpStatusToCategory(pumpStatus int) string {
	switch pumpStatus {
	case 1:
		return "new_creation"
	case 2:
		return "completing"
	case 4:
		return "completed"
	default:
		return "new_creation"
	}
}

func (tws *TokenWebSocketServer) isSubscribedToCategory(client *TokenClient, category string) bool {
	for _, cat := range client.categories {
		if cat == category {
			return true
		}
	}
	return false
}

// HandleKlineWebSocket upgrades a connection and subscribes it to real-time
// K-line updates for one pair (?pair_address=...&chain_id=...).
func (tws *TokenWebSocketServer) HandleKlineWebSocket(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	pairAddress := r.URL.Query().Get("pair_address")
	if pairAddress == "" {
		http.Error(w, `{"error": "Missing required parameter: pair_address"}`, http.StatusBadRequest)
		return
	}

	chainIdStr := r.URL.Query().Get("chain_id")
	if chainIdStr == "" {
		chainIdStr = "100000" // Default to SOL chain
	}
	chainId, err := strconv.ParseInt(chainIdStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid chain_id"}`, http.StatusBadRequest)
		return
	}

	conn, err := tws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logx.Errorf("🔌 [KLINE WS] WebSocket upgrade failed: %v", err)
		return
	}

	client := &KlineClient{
		conn:        conn,
		pairAddress: pairAddress,
		chainId:     chainId,
		send:        make(chan []byte, 256),
	}

	tws.klineMu.Lock()
	tws.klineClients[conn] = client
	tws.klineMu.Unlock()

	logx.Infof("🎯 [KLINE WS] New kline WebSocket client connected for pair: %s, chain: %d", pairAddress, chainId)

	go tws.klineWriteLoop(client)
	go tws.klineReadLoop(client)
}

func (tws *TokenWebSocketServer) klineWriteLoop(client *KlineClient) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				logx.Errorf("📤 [KLINE WS] Error writing to client for pair %s: %v", client.pairAddress, err)
				return
			}
		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (tws *TokenWebSocketServer) klineReadLoop(client *KlineClient) {
	defer func() {
		tws.removeKlineClient(client.conn)
		client.conn.Close()
	}()

	client.conn.SetPongHandler(func(string) error { return nil })

	for {
		if _, _, err := client.conn.ReadMessage(); err != nil {
			logx.Infof("🔌 [KLINE WS] Client disconnected for pair: %s", client.pairAddress)
			break
		}
	}
}

func (tws *TokenWebSocketServer) removeKlineClient(conn *websocket.Conn) {
	tws.klineMu.Lock()
	defer tws.klineMu.Unlock()

	if client, ok := tws.klineClients[conn]; ok {
		close(client.send)
		delete(tws.klineClients, conn)
	}
}

// BroadcastKlineUpdate fans a kline update out to every connected client
// subscribed to that pair.
func (tws *TokenWebSocketServer) BroadcastKlineUpdate(klineData *KlineData) {
	message := &KlineUpdateMessage{Type: "kline_update", Data: klineData}
	data, err := json.Marshal(message)
	if err != nil {
		logx.Errorf("❌ [KLINE WS] Error marshaling kline data: %v", err)
		return
	}

	tws.klineMu.RLock()
	defer tws.klineMu.RUnlock()

	for conn, client := range tws.klineClients {
		if client.pairAddress != klineData.PairAddress {
			continue
		}
		select {
		case client.send <- data:
		default:
			logx.Errorf("❌ [KLINE WS] Client buffer full, dropping client for pair: %s", client.pairAddress)
			go func(c *websocket.Conn) {
				tws.removeKlineClient(c)
				c.Close()
			}(conn)
		}
	}
}

// StartKlineSubscription listens on Redis "kline:updates" (published by
// dataflow) and broadcasts each update to matching WebSocket clients.
func (tws *TokenWebSocketServer) StartKlineSubscription(ctx context.Context) {
	pubsub := tws.redisClient.Subscribe(ctx, "kline:updates")
	defer pubsub.Close()

	if _, err := pubsub.Receive(ctx); err != nil {
		logx.Errorf("❌ [KLINE SUB] Failed to confirm subscription: %v", err)
		return
	}
	logx.Info("✅ [KLINE SUB] Subscribed to 'kline:updates' channel")

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			logx.Info("🛑 [KLINE SUB] Kline subscription stopped")
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}
			var klineData KlineData
			if err := json.Unmarshal([]byte(msg.Payload), &klineData); err != nil {
				logx.Errorf("❌ [KLINE SUB] Error unmarshaling kline data: %v", err)
				continue
			}
			tws.BroadcastKlineUpdate(&klineData)
		}
	}
}

// StartTokenSubscription listens for Redis token updates and broadcasts them
func (tws *TokenWebSocketServer) StartTokenSubscription(ctx context.Context) {
	// Subscribe to Redis channels for token updates
	pubsub := tws.redisClient.Subscribe(ctx, "pump_token_new", "pump_token_update")
	defer pubsub.Close()

	logx.Info("🔔 [TOKEN WS] Started token subscription service")

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			logx.Info("🛑 [TOKEN WS] Token subscription service stopped")
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}

			var tokenData TokenData
			if err := json.Unmarshal([]byte(msg.Payload), &tokenData); err != nil {
				logx.Errorf("❌ [TOKEN WS] Error unmarshaling token data: %v", err)
				continue
			}

			fmt.Println("tokenData name is:", tokenData.TokenName)

			switch msg.Channel {
			case "pump_token_new":
				logx.Infof("🆕 [TOKEN WS] Broadcasting new token: %s", tokenData.TokenName)
				tws.BroadcastNewToken(&tokenData)
			case "pump_token_update":
				logx.Infof("🔄 [TOKEN WS] Broadcasting token update: %s", tokenData.TokenName)
				tws.BroadcastTokenStatusUpdate(&tokenData)
			}
		}
	}
}

func main() {
	// 解析命令行参数
	var configFile = flag.String("f", "etc/websocket.yaml", "the config file")
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化 token WebSocket 服务器
	tokenServer := NewTokenWebSocketServer(cfg)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start token subscription service in a separate goroutine
	go tokenServer.StartTokenSubscription(ctx)
	// Start kline subscription service in a separate goroutine
	go tokenServer.StartKlineSubscription(ctx)

	// Create HTTP server
	mux := http.NewServeMux()

	// Handle token WebSocket connections
	mux.HandleFunc("/ws/tokens", tokenServer.HandleTokenWebSocket)
	// Handle kline WebSocket connections
	mux.HandleFunc("/ws/kline", tokenServer.HandleKlineWebSocket)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy", "service": "token-websocket"}`))
	})

	// 使用配置中的主机和端口
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start HTTP server in a separate goroutine
	go func() {
		logx.Infof("🚀 Token WebSocket server starting on %s...", addr)
		logx.Infof("🔗 WebSocket endpoint: ws://%s/ws/tokens?chain_id=100000", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logx.Info("🛑 Shutting down token WebSocket server...")

	// Cancel context to stop subscription service
	cancel()

	// Shutdown HTTP server with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Token WebSocket server forced to shutdown: %v", err)
	}

	logx.Info("✅ Token WebSocket server exited gracefully")
}
