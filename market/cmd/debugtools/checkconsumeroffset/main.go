package main

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

// 检查 Consumer Group 的 offset 状态
func main() {
	broker := "115.159.107.189:9093"
	topic := "web3fun"
	groupID := "web3-dex-fix-2025-market" // 使用实际的 group ID
	username := "alikafka_post-cn-zp54bjj8x004"
	password := "htIJB7fFfcXbNVnH8rWLD1m52gfkOQI1"

	fmt.Printf("🔍 Checking Consumer Group Offset Status\n")
	fmt.Printf("  Broker: %s\n", broker)
	fmt.Printf("  Topic: %s\n", topic)
	fmt.Printf("  Consumer Group: %s\n", groupID)
	fmt.Println()

	// 配置 SASL
	mechanism := plain.Mechanism{
		Username: username,
		Password: password,
	}

	dialer := &kafka.Dialer{
		Timeout:       10 * time.Second,
		DualStack:     true,
		SASLMechanism: mechanism,
	}

	// 连接 broker
	conn, err := dialer.DialContext(context.Background(), "tcp", broker)
	if err != nil {
		fmt.Printf("❌ Failed to connect: %v\n", err)
		return
	}
	defer conn.Close()

	// 读取 partitions
	partitions, err := conn.ReadPartitions(topic)
	if err != nil {
		fmt.Printf("❌ Failed to read partitions: %v\n", err)
		return
	}

	fmt.Printf("📊 Topic '%s' has %d partition(s)\n\n", topic, len(partitions))

	// 创建 consumer 来检查 offset
	readerConfig := kafka.ReaderConfig{
		Brokers:  []string{broker},
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,    // 最小 1 byte，尽快返回
		MaxBytes: 10e6, // 10MB
		Dialer:   dialer,
	}

	reader := kafka.NewReader(readerConfig)
	defer reader.Close()

	fmt.Printf("📊 Found %d partition(s)\n", len(partitions))

	// 尝试读取消息（使用 StartOffset=FirstOffset 确保从头开始）
	fmt.Println("\n🔍 Testing message consumption with StartOffset=FirstOffset...")
	readerConfig.StartOffset = kafka.FirstOffset
	reader2 := kafka.NewReader(readerConfig)
	defer reader2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msg, err := reader2.ReadMessage(ctx)
	if err != nil {
		if err == context.DeadlineExceeded {
			fmt.Printf("⚠️  No message received within 5 seconds\n")
			fmt.Printf("\n💡 分析：\n")
			fmt.Printf("   1. Topic 中可能真的没有消息\n")
			fmt.Printf("   2. 或者 Consumer Group '%s' 的 offset 已经消费完了所有消息\n", groupID)
			fmt.Printf("   3. 需要确认 consumer 服务是否在发送消息\n")
			fmt.Printf("\n🔧 建议：\n")
			fmt.Printf("   1. 检查 consumer 服务是否运行并发送消息\n")
			fmt.Printf("   2. 使用全新的 Consumer Group 名称测试\n")
			fmt.Printf("   3. 确认 consumer 服务的日志中有 'SendTx success'\n")
		} else {
			fmt.Printf("❌ Error: %v\n", err)
		}
		return
	}

	fmt.Printf("✅ Successfully received message!\n")
	fmt.Printf("   Partition: %d\n", msg.Partition)
	fmt.Printf("   Offset: %d\n", msg.Offset)
	fmt.Printf("   Key: %s\n", string(msg.Key))
	fmt.Printf("   Value length: %d bytes\n", len(msg.Value))
}
