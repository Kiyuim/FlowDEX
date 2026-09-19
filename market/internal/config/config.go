package config

import (
	datakline "dex/market/market"

	"github.com/SpectatorNan/gorm-zero/gormc/config/mysql"
	"github.com/chengfield/go-queue/kq"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Mysql       MysqlConf
	Redis       redis.RedisConf `json:"BizRedis"`
	KqSolTrades kq.KqConf       `json:"KqSolTrades,optional"`
	// KqSolTrades.Username/Password are third-party fields go-zero can't env-bind
	// directly, so the real Kafka SASL credentials are injected via these env-only
	// siblings (Railway secrets) instead of living in the committed yaml.
	KafkaUsernameEnv string `json:"KafkaUsernameEnv,optional,env=KAFKA_USERNAME"`
	KafkaPasswordEnv string `json:"KafkaPasswordEnv,optional,env=KAFKA_PASSWORD"`
}

// ApplyEnvOverrides folds env-injected secrets that go-zero cannot bind directly
// (fields on third-party structs) into the loaded config. Call once, right after
// conf.MustLoad.
func (c *Config) ApplyEnvOverrides() {
	if v := c.KafkaUsernameEnv; v != "" {
		c.KqSolTrades.Username = v
	}
	if v := c.KafkaPasswordEnv; v != "" {
		c.KqSolTrades.Password = v
	}
}

type MysqlConf struct {
	Master mysql.Mysql   `json:"Master"`
	Slave  []mysql.Mysql `json:"Slave,optional"`
}

var KlineProcessedCh = make(chan *datakline.Kline, 10000)
