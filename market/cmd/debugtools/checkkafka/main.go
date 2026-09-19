package main

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

// 测试 Kafka 连接的独立程序
func main() {
	broker := "115.159.107.189:9093"
	topic := "web3fun"
	groupID := "web3-dex-fix-2025-market-test"
	username := "alikafka_post-cn-zp54bjj8x004"
	password := "htIJB7fFfcXbNVnH8rWLD1m52gfkOQI1"

	fmt.Printf("🔍 Testing Kafka connection...\n")
	fmt.Printf("  Broker: %s\n", broker)
	fmt.Printf("  Topic: %s\n", topic)
	fmt.Printf("  Group: %s\n", groupID)
	fmt.Printf("  Username: %s\n", username)
	fmt.Println()

	// 配置 SASL
	mechanism := plain.Mechanism{
		Username: username,
		Password: password,
	}

	// 创建 dialer
	dialer := &kafka.Dialer{
		Timeout:       10 * time.Second,
		DualStack:     true,
		SASLMechanism: mechanism,
	}

	// 测试连接
	fmt.Println("1. Testing broker connection...")
	conn, err := dialer.DialContext(context.Background(), "tcp", broker)
	if err != nil {
		fmt.Printf("❌ Failed to connect to broker: %v\n", err)
		return
	}
	defer conn.Close()
	fmt.Println("✅ Successfully connected to broker")

	// 测试读取 metadata
	fmt.Println("\n2. Testing metadata retrieval...")
	controller, err := conn.Controller()
	if err != nil {
		fmt.Printf("❌ Failed to get controller: %v\n", err)
		return
	}
	fmt.Printf("✅ Controller: %s:%d\n", controller.Host, controller.Port)

	// 测试创建 consumer
	fmt.Println("\n3. Testing consumer creation...")
	readerConfig := kafka.ReaderConfig{
		Brokers:  []string{broker},
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
		Dialer:   dialer,
	}

	reader := kafka.NewReader(readerConfig)
	defer reader.Close()

	fmt.Println("✅ Consumer created successfully")

	// 检查 Topic 的 partition 信息
	fmt.Println("\n4. Checking topic partitions...")
	partitions, err := conn.ReadPartitions(topic)
	if err != nil {
		fmt.Printf("❌ Failed to read partitions: %v\n", err)
		return
	}
	fmt.Printf("✅ Topic has %d partition(s)\n", len(partitions))

	for _, p := range partitions {
		fmt.Printf("   Partition %d: Leader=%s:%d\n", p.ID, p.Leader.Host, p.Leader.Port)
	}

	// 尝试读取一条消息（带超时）
	fmt.Println("\n5. Testing message consumption (timeout: 10s)...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	msg, err := reader.ReadMessage(ctx)
	if err != nil {
		if err == context.DeadlineExceeded {
			fmt.Printf("⚠️  No message received within 10 seconds\n")
			fmt.Printf("\n可能的原因：\n")
			fmt.Printf("   1. Topic 中真的没有消息（需要 consumer 服务发送消息）\n")
			fmt.Printf("   2. Consumer Group '%s' 的 offset 已经消费完了所有消息\n", groupID)
			fmt.Printf("   3. 消息被其他消费者消费了\n")
			fmt.Printf("\n建议：\n")
			fmt.Printf("   1. 检查 consumer 服务是否在发送消息\n")
			fmt.Printf("   2. 使用新的 Consumer Group 名称（避免 offset 冲突）\n")
			fmt.Printf("   3. 确认 consumer 服务正在运行\n")
		} else {
			fmt.Printf("❌ Error reading message: %v\n", err)
		}
		return
	}

	fmt.Printf("✅ Successfully received message!\n")
	fmt.Printf("   Topic: %s\n", msg.Topic)
	fmt.Printf("   Partition: %d\n", msg.Partition)
	fmt.Printf("   Offset: %d\n", msg.Offset)
	fmt.Printf("   Key: %s\n", string(msg.Key))
	fmt.Printf("   Value length: %d bytes\n", len(msg.Value))
}
