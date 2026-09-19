package main

import (
	"dex/market/internal/config"
	"flag"
	"fmt"

	"github.com/zeromicro/go-zero/core/conf"
)

// 测试配置是否正确加载
func main() {
	configFile := flag.String("f", "etc/market.yaml", "the config file")
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	fmt.Println("==========================================")
	fmt.Println("🔍 检查配置加载")
	fmt.Println("==========================================")
	fmt.Println()

	fmt.Printf("Kafka 配置:\n")
	fmt.Printf("  Brokers: %v\n", c.KqSolTrades.Brokers)
	fmt.Printf("  Topic: %s\n", c.KqSolTrades.Topic)
	fmt.Printf("  Group: %s\n", c.KqSolTrades.Group)
	fmt.Printf("  Offset: %s\n", c.KqSolTrades.Offset)
	fmt.Printf("  Username: %s\n", c.KqSolTrades.Username)
	fmt.Printf("  Password: %s\n", func() string {
		if c.KqSolTrades.Password != "" {
			return "***" + c.KqSolTrades.Password[len(c.KqSolTrades.Password)-4:]
		}
		return ""
	}())
	fmt.Println()

	if len(c.KqSolTrades.Brokers) == 0 {
		fmt.Println("❌ Kafka Brokers 为空")
	} else {
		fmt.Printf("✅ Kafka Brokers 配置: %d 个\n", len(c.KqSolTrades.Brokers))
	}

	if c.KqSolTrades.Topic == "" {
		fmt.Println("❌ Topic 为空")
	} else {
		fmt.Printf("✅ Topic: %s\n", c.KqSolTrades.Topic)
	}

	if c.KqSolTrades.Group == "" {
		fmt.Println("❌ Consumer Group 为空")
	} else {
		fmt.Printf("✅ Consumer Group: %s\n", c.KqSolTrades.Group)
	}

	if c.KqSolTrades.Offset == "" {
		fmt.Println("⚠️  Offset 为空（将使用默认值）")
	} else {
		fmt.Printf("✅ Offset: %s\n", c.KqSolTrades.Offset)
	}
}
