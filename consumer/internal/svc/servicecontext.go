package svc

import (
	"dex/consumer/internal/config"
	"dex/market/marketclient"
	"dex/model/migration"
	"dex/model/solmodel"
	"dex/trade/tradeclient"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/blocto/solana-go-sdk/client"
	solclient "github.com/blocto/solana-go-sdk/client"
	"github.com/blocto/solana-go-sdk/rpc"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type ServiceContext struct {
	Config                    config.Config
	MarketService             marketclient.Market
	TradeService              tradeclient.Trade
	solClientLock             sync.Mutex
	solClientIndex            int
	solClient                 *solclient.Client
	solClients                []*solclient.Client
	TokenModel                solmodel.TokenModel
	TradeModel                solmodel.TradeModel
	PairModel                 solmodel.PairModel
	SolAccountModel           solmodel.SolAccountModel
	SolRaydiumCLMMPoolV1Model solmodel.ClmmPoolInfoV1Model
	SolRaydiumCLMMPoolV2Model solmodel.ClmmPoolInfoV2Model
	SolRaydiumCPMMPoolModel   solmodel.CpmmPoolInfoModel
	BlockModel                solmodel.BlockModel
	PumpAmmInfoModel          solmodel.PumpAmmInfoModel
	SolRaydiumPoolModel       solmodel.RaydiumPoolModel
	SolTokenAccountModel      solmodel.SolTokenAccountModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config: c,
	}
}

func NewSolServiceContext(c config.Config) *ServiceContext {
	logx.MustSetup(c.Log)

	logx.Infof("newSolServiceContext: config:%#v", c)

	// timeout=10s bounds each dial attempt so a blocked DB (e.g. IP allowlist)
	// fails fast instead of hanging ~2 min on the TCP SYN.
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&timeout=10s", c.MySQLConfig.User, c.MySQLConfig.Password, c.MySQLConfig.Host, c.MySQLConfig.Port, c.MySQLConfig.DBName)
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second * 3, // Slow SQL threshold
			LogLevel:                  logger.Warn,     // Log level
			IgnoreRecordNotFoundError: true,            // Ignore ErrRecordNotFound error for logger
			ParameterizedQueries:      false,           // Don't include params in the SQL log
			Colorful:                  true,
		},
	)
	// Retry the MySQL connection with backoff instead of fatally exiting, so the
	// consumer stays up and self-heals once the DB becomes reachable (e.g. after
	// the Tencent CDB IP allowlist is opened) — no redeploy required.
	var db *gorm.DB
	var err error
	for attempt := 1; ; attempt++ {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: gormLogger})
		if err == nil {
			if sqlDB, pingErr := db.DB(); pingErr == nil {
				if pingErr = sqlDB.Ping(); pingErr == nil {
					logx.Infof("connected to mysql after %d attempt(s): %s:%d", attempt, c.MySQLConfig.Host, c.MySQLConfig.Port)
					break
				} else {
					err = pingErr
				}
			} else {
				err = pingErr
			}
		}
		logx.Errorf("connect to mysql failed (attempt %d): %v; retrying in 10s. host=%s:%d", attempt, err, c.MySQLConfig.Host, c.MySQLConfig.Port)
		time.Sleep(10 * time.Second)
	}

	// Self-provision the core schema (idempotent) so a fresh database — e.g. the
	// Railway-internal MySQL — comes up with all tables without manual DB access.
	if err := migration.EnsureCoreSchema(db); err != nil {
		logx.Errorf("EnsureCoreSchema error (continuing): %v", err)
	} else {
		logx.Infof("core schema ensured")
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxIdleConns(200)
	sqlDB.SetMaxOpenConns(500)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	var solClients []*solclient.Client
	for _, node := range config.Cfg.Sol.NodeUrl {
		client.New(rpc.WithEndpoint(node), rpc.WithHTTPClient(&http.Client{
			Timeout: 10 * time.Second,
		}))
		solClients = append(solClients, client.NewClient(node))
	}

	return &ServiceContext{
		Config:                    c,
		MarketService:             marketclient.NewMarket(zrpc.MustNewClient(c.MarketService)),
		BlockModel:                solmodel.NewBlockModel(db),
		TokenModel:                solmodel.NewTokenModel(db),
		TradeModel:                solmodel.NewTradeModel(db),
		PairModel:                 solmodel.NewPairModel(db),
		TradeService:              tradeclient.NewTrade(zrpc.MustNewClient(c.TradeService)),
		SolTokenAccountModel:      solmodel.NewSolTokenAccountModel(db),
		PumpAmmInfoModel:          solmodel.NewPumpAmmInfoModel(db),
		SolRaydiumCLMMPoolV1Model: solmodel.NewClmmPoolInfoV1Model(db),
		SolRaydiumCLMMPoolV2Model: solmodel.NewClmmPoolInfoV2Model(db),
		solClients:                solClients,
	}
}

func (sc *ServiceContext) GetSolClient() *client.Client {
	sc.solClientLock.Lock()
	defer sc.solClientLock.Unlock()
	sc.solClientIndex++
	index := sc.solClientIndex % len(sc.solClients)
	sc.solClient = sc.solClients[index]
	return sc.solClients[index]
}
