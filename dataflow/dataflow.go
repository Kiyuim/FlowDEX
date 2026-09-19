package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"dex/dataflow/dataflow"
	"dex/dataflow/internal/config"
	"dex/dataflow/internal/server"
	"dex/dataflow/internal/svc"
	rds "dex/dataflow/pkg/redis"

	"dex/dataflow/internal/mqs"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/dataflow.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	c.ApplyEnvOverrides()
	ctx := svc.NewServiceContext(c)
	rds.Init(&redis.RedisKeyConf{
		RedisConf: redis.RedisConf{
			Host:        c.Redis.Host,
			Type:        c.Redis.Type,
			Pass:        c.Redis.Pass,
			Tls:         c.Redis.Tls,
			PingTimeout: c.Redis.PingTimeout,
		},
	})
	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		dataflow.RegisterDataflowServer(grpcServer, server.NewDataflowServer(ctx))
		reflection.Register(grpcServer)
	})
	defer s.Stop()

	serviceGroup := service.NewServiceGroup()
	defer serviceGroup.Stop()
	for _, mq := range mqs.TradeConsumers(c, context.Background(), ctx) {
		serviceGroup.Add(mq)
	}

	go func() {
		time.Sleep(10 * time.Second)
		serviceGroup.Start()
	}()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
