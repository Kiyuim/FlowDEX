package config

import (
	"strings"

	"dex/pkg/constants"

	"github.com/chengfield/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
)

var Cfg Config

var (
	SolRpcUseFrequency int
)

type Config struct {
	zrpc.RpcServerConf
	// database
	MySQLConfig      MySQLConfig        `json:"Mysql"`
	MarketService    zrpc.RpcClientConf `json:"market_service"`
	WebsocketService zrpc.RpcClientConf `json:"websocket_service"`
	TradeService     zrpc.RpcClientConf `json:"trade_service"`

	KqSolTrades kq.KqConf `json:"KqSolTrades,optional"`

	Consumer Consumer `json:"Consumer,optional"`

	Sol Chain `json:"Sol,optional"`

	// KqSolTrades.Username/Password are third-party fields go-zero can't env-bind
	// directly, so the real Kafka SASL credentials are injected via these env-only
	// siblings (Railway secrets) instead of living in the committed yaml.
	KafkaUsernameEnv string `json:"KafkaUsernameEnv,optional,env=KAFKA_USERNAME"`
	KafkaPasswordEnv string `json:"KafkaPasswordEnv,optional,env=KAFKA_PASSWORD"`
}

// ApplyEnvOverrides folds env-injected values that go-zero cannot bind directly
// (slice fields, third-party struct fields) into the loaded config. Call once,
// right after conf.MustLoad.
func (c *Config) ApplyEnvOverrides() {
	if v := c.KafkaUsernameEnv; v != "" {
		c.KqSolTrades.Username = v
	}
	if v := c.KafkaPasswordEnv; v != "" {
		c.KqSolTrades.Password = v
	}
	if s := strings.TrimSpace(c.Sol.NodeUrlEnv); s != "" {
		var urls []string
		for _, part := range strings.Split(s, ",") {
			if u := strings.TrimSpace(part); u != "" {
				urls = append(urls, u)
			}
		}
		if len(urls) > 0 {
			c.Sol.NodeUrl = urls
		}
	}
}

type Consumer struct {
	Concurrency int `json:"Concurrency,env=CONSUMER_CONCURRENCY"`
}

// env overrides belong INSIDE the single json tag as a field option
// (e.g. `json:"Host,env=MYSQL_HOST"`); a second `json:",env=..."` tag is
// silently ignored by Go reflection. MySQL is intentionally left yaml-driven
// (env tags kept inert as the double-tag form) because the live Railway
// services still carry a stale MYSQL_PASSWORD var whose value is wrong for the
// current DB — activating it would break the connection. Re-merge to single
// tags once that stale var is cleared.
type MySQLConfig struct {
	User     string `json:"User"     json:",env=MYSQL_USER"`
	Password string `json:"Password" json:",env=MYSQL_PASSWORD"`
	Host     string `json:"Host"     json:",env=MYSQL_HOST"`
	Port     int    `json:"Port"     json:",env=MYSQL_PORT"`
	DBName   string `json:"DBname"   json:",env=MYSQL_DBNAME"`
}

type Chain struct {
	ChainId    int64    `json:"ChainId,env=SOL_CHAINID"`
	NodeUrl    []string `json:"NodeUrl"`
	MEVNodeUrl string   `json:"MevNodeUrl,optional,env=SOL_MEVNODEURL"`
	WSUrl      string   `json:"WSUrl,optional,env=SOL_WSURL"`
	StartBlock uint64   `json:"StartBlock,optional,env=SOL_STARTBLOCK"`
	// Network selects the Solana cluster ("devnet"/"mainnet"); inferred from
	// NodeUrl when empty (see constants.IsDevnet).
	Network string `json:"Network,optional,env=SOL_NETWORK"`
	// NodeUrlEnv injects the RPC endpoint(s) from the SOL_NODE_URL env var (e.g. a
	// Railway secret) so a Helius API key never lives in git. Comma-separated;
	// go-zero cannot env-bind a []string, so ApplyEnvOverrides folds it in.
	NodeUrlEnv string `json:"NodeUrlEnv,optional,env=SOL_NODE_URL"`
}

func SaveConf(cf Config) {
	Cfg = cf
}

func FindChainRpcByChainId(chainId int) (rpc string) {
	var rpcs []string
	var useFrequency *int

	switch chainId {
	case constants.SolChainIdInt:
		rpcs = Cfg.Sol.NodeUrl
		useFrequency = &SolRpcUseFrequency
	default:
		logx.Error("No Rpc Config")
		return
	}

	if len(rpcs) == 0 {
		logx.Error("No Rpc Config")
		return
	}

	*useFrequency++
	index := *useFrequency % len(rpcs)
	rpc = rpcs[index]
	return
}
