package main

import (
	"flag"
	"fmt"
	"os"

	"dex/pkg/disruptorx"
	"dex/trade/internal/config"
	"dex/trade/internal/server"
	"dex/trade/internal/svc"
	"dex/trade/internal/ticker"
	"dex/trade/pkg/entity"
	"dex/trade/trade"

	"dex/trade/internal/proclimitorder/tokenpricelimit"
	"dex/trade/internal/proclimitorder/trailingstop"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/proc"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/trade.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	c.ApplyEnvOverrides() // fold SOL_NODE_URL (Railway secret) into Sol.NodeUrl
	ctx := svc.NewServiceContext(c)
	ctx.DisruptorWrapper = newDisruptor(ctx)

	// Confirms on-chain fills against the consumer-built trade tables and
	// auto-creates double-out sell orders (lesson 7.13).
	tradeTicker := ticker.NewTradeTicker(ctx)
	go tradeTicker.Start()
	proc.AddShutdownListener(tradeTicker.Stop)

	srv := server.NewTradeServer(ctx)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		trade.RegisterTradeServer(grpcServer, srv)
		reflection.Register(grpcServer)
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	fmt.Fprintf(os.Stderr, "🚀 DEBUG: Server starting with config: %+v\n", c)
	fmt.Fprintf(os.Stderr, "🚀 DEBUG: ListenOn: %s\n", c.ListenOn)
	s.Start()
}

func newDisruptor(sc *svc.ServiceContext) *disruptorx.DisruptorWrapper[*entity.OrderMessage] {
	// Define buffer size (must be a power of 2)
	bufferSize := int64(1024 * 32)
	trailingStopSubscriber := trailingstop.NewTrailingStopSubscriber(sc)
	limitSubscriber := tokenpricelimit.NewLimitSubscriber(sc)

	// Create multiple consumer groups
	consumers := []disruptorx.Consumer[*entity.OrderMessage]{
		trailingStopSubscriber,
		limitSubscriber,
	}

	// Create the DisruptorWrapper with string type as the data type
	disruptorWrapper, err := disruptorx.NewDisruptorWrapper[*entity.OrderMessage](bufferSize, consumers...)
	if err != nil {
		logx.Must(err)
		return nil
	}
	// Start the disruptor, which will start consuming messages in separate goroutines
	go disruptorWrapper.Start()
	proc.AddShutdownListener(func() {
		disruptorWrapper.Close()
	})
	return disruptorWrapper
}
