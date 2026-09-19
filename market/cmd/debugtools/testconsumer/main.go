package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"dex/market/internal/config"
	"dex/market/internal/constants"
	"dex/market/internal/mqs/consumers"
	"dex/market/internal/svc"

	"github.com/chengfield/go-queue/kq"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
)

// 直接测试 Consumer 是否能工作
func main() {
	configFile := flag.String("f", "etc/market.yaml", "the config file")
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	fmt.Println("==========================================")
	fmt.Println("🔍 直接测试 Kafka Consumer")
	fmt.Println("==========================================")
	fmt.Println()

	fmt.Printf("配置信息:\n")
	fmt.Printf("  Brokers: %v\n", c.KqSolTrades.Brokers)
	fmt.Printf("  Topic: %s\n", c.KqSolTrades.Topic)
	fmt.Printf("  Group: %s\n", c.KqSolTrades.Group)
	fmt.Printf("  Offset: %s\n", c.KqSolTrades.Offset)
	fmt.Printf("  MinBytes: %d\n", c.KqSolTrades.MinBytes)
	fmt.Println()

	// 设置 MinBytes = 0
	c.KqSolTrades.MinBytes = 0

	// 创建 ServiceContext（简化版）
	svcCtx := svc.NewServiceContext(c)

	// 创建 Consumer
	fmt.Println("创建 TradeConsumer...")
	ctx := context.Background()
	consumer := consumers.NewTradeConsumer(ctx, svcCtx, constants.Sol)
	fmt.Println("✅ TradeConsumer 创建成功")
	fmt.Println()

	// 创建 Queue
	fmt.Println("创建 Kafka Queue...")
	queue := kq.MustNewQueue(c.KqSolTrades, consumer)
	fmt.Println("✅ Kafka Queue 创建成功")
	fmt.Println()

	// 创建 ServiceGroup 并启动
	fmt.Println("启动 ServiceGroup...")
	serviceGroup := service.NewServiceGroup()
	serviceGroup.Add(queue)

	// 在 goroutine 中启动（模拟 market.go 的方式）
	go func() {
		fmt.Println("🚀 ServiceGroup.Start() 调用中...")
		serviceGroup.Start()
		fmt.Println("⚠️  ServiceGroup 已停止")
	}()

	// 等待一段时间让 Consumer 启动
	fmt.Println("等待 Consumer 启动和连接...")
	time.Sleep(3 * time.Second)
	fmt.Println()

	fmt.Println("==========================================")
	fmt.Println("✅ 测试 Consumer 已启动")
	fmt.Println("==========================================")
	fmt.Println()
	fmt.Println("💡 现在等待消息...")
	fmt.Println("   如果 Consumer 服务发送消息，应该会看到:")
	fmt.Println("   🔥 Consume called! Message received from Kafka")
	fmt.Println()
	fmt.Println("   按 Ctrl+C 停止测试")
	fmt.Println()

	// 保持运行
	select {}
}
