package config

import (
	"os"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

// Verifies ApplyEnvOverrides in isolation (deterministic, no env plumbing).
func TestApplyEnvOverrides_Unit(t *testing.T) {
	var c Config
	c.SolConfig.NodeUrl = []string{"https://api.devnet.solana.com"}

	// empty NodeUrlEnv -> NodeUrl unchanged
	c.ApplyEnvOverrides()
	if len(c.SolConfig.NodeUrl) != 1 || c.SolConfig.NodeUrl[0] != "https://api.devnet.solana.com" {
		t.Fatalf("empty env must not change NodeUrl, got %#v", c.SolConfig.NodeUrl)
	}

	// set NodeUrlEnv (as go-zero would) -> replace + comma-split + trim
	c.SolConfig.NodeUrlEnv = " https://devnet.helius-rpc.com/?api-key=K , https://backup "
	c.ApplyEnvOverrides()
	if len(c.SolConfig.NodeUrl) != 2 ||
		c.SolConfig.NodeUrl[0] != "https://devnet.helius-rpc.com/?api-key=K" ||
		c.SolConfig.NodeUrl[1] != "https://backup" {
		t.Fatalf("override failed: %#v", c.SolConfig.NodeUrl)
	}
}

// Verifies the whole path: SOL_NODE_URL env -> loaded into NodeUrlEnv by go-zero
// -> folded into NodeUrl by ApplyEnvOverrides (mirrors production main()).
func TestSolNodeUrlEnv_EndToEnd(t *testing.T) {
	const y = `
Name: trade.rpc
ListenOn: 0.0.0.0:8081
Mysql:
  User: root
  Password: x
  Host: 127.0.0.1
  Port: 3306
  Dbname: fun_dexs
market_service:
  target: localhost:8080
  nonblock: true
Sol:
  ChainId: 100000
  Enable: true
  NodeUrl:
    - "https://api.devnet.solana.com"
`
	os.Setenv("SOL_NODE_URL", "https://devnet.helius-rpc.com/?api-key=SECRET")
	defer os.Unsetenv("SOL_NODE_URL")

	var c Config
	if err := conf.LoadFromYamlBytes([]byte(y), &c); err != nil {
		t.Fatalf("load: %v", err)
	}
	t.Logf("loaded: NodeUrlEnv=%q NodeUrl=%v", c.SolConfig.NodeUrlEnv, c.SolConfig.NodeUrl)
	if c.SolConfig.NodeUrlEnv == "" {
		t.Fatalf("SOL_NODE_URL was not bound into NodeUrlEnv (env tag not applied)")
	}
	c.ApplyEnvOverrides()
	if len(c.SolConfig.NodeUrl) != 1 || c.SolConfig.NodeUrl[0] != "https://devnet.helius-rpc.com/?api-key=SECRET" {
		t.Fatalf("end-to-end override failed: %#v", c.SolConfig.NodeUrl)
	}
}
