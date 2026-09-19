package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"dex/market/internal/cache"
	"dex/market/internal/config"
	"dex/market/internal/mqs"
	"dex/market/internal/server"
	"dex/market/internal/svc"
	"dex/market/internal/ticker"
	"dex/market/market"
	rds "dex/market/pkg/redis"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/market.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	c.ApplyEnvOverrides()
	svcCtx := svc.NewServiceContext(c)
	rds.Init(&redis.RedisKeyConf{
		RedisConf: redis.RedisConf{
			Host:        c.Redis.Host,
			Type:        c.Redis.Type,
			Pass:        c.Redis.Pass,
			Tls:         c.Redis.Tls,
			PingTimeout: c.Redis.PingTimeout,
		},
	})
	cache.Init(svcCtx, nil)
	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		market.RegisterMarketServer(grpcServer, server.NewMarketServer(svcCtx))
		reflection.Register(grpcServer)
	})
	defer s.Stop()

	serviceGroup := service.NewServiceGroup()
	defer serviceGroup.Stop()

	// 添加 Kafka Consumer
	fmt.Printf("🔍 Kafka config check: brokers=%d, topic=%s, group=%s\n",
		len(c.KqSolTrades.Brokers), c.KqSolTrades.Topic, c.KqSolTrades.Group)
	if len(c.KqSolTrades.Brokers) > 0 {
		consumers := mqs.TradeConsumers(c, context.Background(), svcCtx)
		fmt.Printf("✅ Created %d Kafka consumer(s)\n", len(consumers))
		for i, mq := range consumers {
			fmt.Printf("  Adding consumer %d to service group\n", i+1)
			serviceGroup.Add(mq)
		}
	} else {
		fmt.Println("⚠️  No Kafka brokers configured, skipping consumer setup")
	}

	{
		pumpTicker := ticker.NewPumpTicker(svcCtx)
		serviceGroup.Add(pumpTicker)
	}

	fmt.Printf("🚀 Starting service group (includes Kafka consumer)...\n")
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("❌ PANIC in service group: %v\n", r)
			}
		}()
		fmt.Printf("📌 About to call serviceGroup.Start()...\n")
		serviceGroup.Start()
		fmt.Printf("⚠️  Service group stopped\n")
	}()

	// Give service group more time to start and connect to Kafka
	fmt.Printf("⏳ Waiting for service group to start (3 seconds)...\n")
	time.Sleep(3 * time.Second)
	fmt.Printf("✅ Service group should be started now\n")
	fmt.Printf("💡 Consumer should be connected to Kafka and waiting for messages...\n")

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
