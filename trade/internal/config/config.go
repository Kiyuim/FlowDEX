package config

import (
	"strings"

	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	MySQLConfig   MySQLConfig        `json:"Mysql"`
	MarketService zrpc.RpcClientConf `json:"market_service"`
	SolConfig     ChainConfig        `json:"Sol"`
	SimulateOnly  bool               `json:"simulate_only,optional,env=SIMULATE_ONLY"`
}

// NOTE: env overrides must be a field option inside the single json tag.
// A separate `json:",env=..."` tag is ignored by Go reflection.
type ChainConfig struct {
	ChainId uint64   `json:"ChainId,env=SOL_CHAINID"`
	Enable  bool     `json:"Enable,env=SOL_ENABLE"`
	NodeUrl []string `json:"NodeUrl"`
	Jito    string   `json:"Jito,optional,env=SOL_JITO"`
	UUID    string   `json:"UUID,optional,env=SOL_UUID"`
	// Network selects the Solana cluster program IDs ("devnet"/"mainnet").
	// When empty it is inferred from NodeUrl (see constants.IsDevnet).
	Network string `json:"Network,optional,env=SOL_NETWORK"`
	// NodeUrlEnv injects the RPC endpoint(s) from the SOL_NODE_URL env var
	// (e.g. a Railway secret) so a Helius API key never lives in git. Comma-
	// separated; when set it replaces NodeUrl (first URL is primary/tx-build).
	// go-zero cannot env-bind a []string, so ApplyEnvOverrides folds it in.
	NodeUrlEnv string `json:"NodeUrlEnv,optional,env=SOL_NODE_URL"`
}

// ApplyEnvOverrides folds env-injected values that go-zero cannot bind directly
// (e.g. slice fields) into the loaded config. Call once, right after conf.MustLoad.
func (c *Config) ApplyEnvOverrides() {
	if s := strings.TrimSpace(c.SolConfig.NodeUrlEnv); s != "" {
		var urls []string
		for _, part := range strings.Split(s, ",") {
			if u := strings.TrimSpace(part); u != "" {
				urls = append(urls, u)
			}
		}
		if len(urls) > 0 {
			c.SolConfig.NodeUrl = urls
		}
	}
}

// MySQL left yaml-driven (env tags inert) — see the consumer config note: a
// stale, wrong MYSQL_PASSWORD var on the live services must be cleared before
// these can be safely merged to single `json:"...,env=..."` tags.
type MySQLConfig struct {
	User     string `json:"User"     json:",env=MYSQL_USER"`
	Password string `json:"Password" json:",env=MYSQL_PASSWORD"`
	Host     string `json:"Host"     json:",env=MYSQL_HOST"`
	Port     int    `json:"Port"     json:",env=MYSQL_PORT"`
	DBName   string `json:"DBname"   json:",env=MYSQL_DBNAME"`
}
