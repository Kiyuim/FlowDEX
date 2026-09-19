package svc

import (
	"dex/market/market"
	"dex/pkg/constants"
	"dex/pkg/disruptorx"
	"dex/pkg/raydium/clmm"
	"dex/pkg/raydium/cpmm"
	"dex/trade/internal/config"
	"dex/trade/pkg/entity"
	"fmt"
	"os"
	"time"

	"dex/trade/internal/chain/solana"

	"github.com/panjf2000/ants/v2"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/proc"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type ServiceContext struct {
	Config           config.Config
	Redis            *redis.Redis
	DisruptorWrapper *disruptorx.DisruptorWrapper[*entity.OrderMessage]
	Pool             *ants.Pool
	SolTxMananger    *solana.TxManager
	MarketClient     market.MarketClient

	DB *gorm.DB
}

func NewServiceContext(c config.Config) *ServiceContext {
	redisService := c.Redis.NewRedis()
	// Initialize database connection
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		c.MySQLConfig.User,
		c.MySQLConfig.Password,
		c.MySQLConfig.Host,
		c.MySQLConfig.Port,
		c.MySQLConfig.DBName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		panic(fmt.Sprintf("connect to mysql error: %v, dsn: %v", err, dsn))
	}

	marketClient := market.NewMarketClient(zrpc.MustNewClient(c.MarketService).Conn())

	pool, err := ants.NewPool(0)
	if err != nil {
		logx.Error("NewPool error:", err)
		os.Exit(1)
	}
	// 携程池的退出逻辑
	proc.AddShutdownListener(func() {
		logx.Info("stop pool")
		if err := pool.ReleaseTimeout(30 * time.Second); err != nil {
			logx.Errorf("stop pool timeout err:%s , cap is %d", err.Error(), pool.Cap())
		}
	})

	sqlDB, err := db.DB()
	if err != nil {
		panic(err)
	}

	sqlDB.SetMaxOpenConns(500)
	sqlDB.SetMaxIdleConns(200)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	if err = sqlDB.Ping(); err != nil {
		panic(err)
	}

	inst := &ServiceContext{
		Config:       c,
		Redis:        redisService,
		MarketClient: marketClient,
		DB:           db,
		Pool:         pool,
	}

	// Initialize SolTxMananger if enabled
	if c.SolConfig.Enable {
		logx.Infof("SolConfig enabled, initializing SolTxMananger...")
		// Select devnet vs mainnet Raydium program IDs once at startup.
		if constants.IsDevnet(c.SolConfig.Network, c.SolConfig.NodeUrl) {
			clmm.SetDevnetProgramID()
			cpmm.SetDevnetProgramID()
			logx.Infof("devnet detected: Raydium CLMM/CPMM program IDs set to devnet")
		}
		solTxMananger, err := solana.NewTxManager(db, c.SolConfig.NodeUrl[0], c.SolConfig.Jito, c.SolConfig.UUID, c.SimulateOnly)
		if err != nil {
			panic(err)
		}
		inst.SolTxMananger = solTxMananger
		logx.Infof("SolTxMananger initialized successfully")
	} else {
		logx.Infof("SolConfig disabled, SolTxMananger not initialized")
	}

	return inst
}
