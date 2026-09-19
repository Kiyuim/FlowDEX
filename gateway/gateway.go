package main

import (
	protosets "dex/gateway/internal/embed"
	"dex/gateway/middleware"
	"flag"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/gateway"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/gateway.yaml", "config file")

func main() {
	flag.Parse()
	conf.MustLoad(*configFile, &middleware.GlobalConfig)

	path, err := protosets.ExtractProtoSets()
	if err != nil {
		logx.Errorf("extract proto sets failed: %v", err)
		return
	}
	logx.Infof("proto sets path: %s", path)
	protosets.UpdateProtoSetsPaths(&middleware.GlobalConfig.GatewayConf, path)

	gw := gateway.MustNewServer(middleware.GlobalConfig.GatewayConf)
	defer gw.Stop()
	rest.WithNotAllowedHandler(middleware.NewCorsMiddleware().Handler())(gw.Server)
	gw.Use(middleware.NewCorsMiddleware().Handle)
	gw.Use(middleware.HeaderMiddleware)
	gw.Use(middleware.AuthMiddleware)
	gw.Use(middleware.WrapResponse)

	logx.Infof("Starting gateway at %s:%d ...", middleware.GlobalConfig.GatewayConf.Host, middleware.GlobalConfig.GatewayConf.Port)
	gw.Start()
}
