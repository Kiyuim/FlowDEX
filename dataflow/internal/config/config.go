package config

import (
	datakline "dex/dataflow/dataflow"

	"github.com/SpectatorNan/gorm-zero/gormc/config/mysql"
	"github.com/chengfield/go-queue/kq"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	KqSolConf kq.KqConf `json:"KqSol"`
	Mysql     MysqlConf
	// KqSolConf.Username/Password and Mysql.Master.Password/Redis.Pass are all
	// third-party fields go-zero can't env-bind directly, so the real secrets are
	// injected via these env-only siblings (Railway secrets) instead of living in
	// the committed yaml.
	KafkaUsernameEnv string `json:"KafkaUsernameEnv,optional,env=KAFKA_USERNAME"`
	KafkaPasswordEnv string `json:"KafkaPasswordEnv,optional,env=KAFKA_PASSWORD"`
	MysqlPasswordEnv string `json:"MysqlPasswordEnv,optional,env=MYSQL_PASSWORD"`
	RedisPassEnv     string `json:"RedisPassEnv,optional,env=REDIS_PASSWORD"`
}

// ApplyEnvOverrides folds env-injected secrets that go-zero cannot bind directly
// (fields on third-party structs) into the loaded config. Call once, right after
// conf.MustLoad.
func (c *Config) ApplyEnvOverrides() {
	if v := c.KafkaUsernameEnv; v != "" {
		c.KqSolConf.Username = v
	}
	if v := c.KafkaPasswordEnv; v != "" {
		c.KqSolConf.Password = v
	}
	if v := c.MysqlPasswordEnv; v != "" {
		c.Mysql.Master.Password = v
	}
	if v := c.RedisPassEnv; v != "" {
		c.Redis.Pass = v
	}
}

type MysqlConf struct {
	Master mysql.Mysql   `json:"Master"`
	Slave  []mysql.Mysql `json:"Slave,optional"`
}

var KlineProcessedCh = make(chan *datakline.Kline, 10000)
