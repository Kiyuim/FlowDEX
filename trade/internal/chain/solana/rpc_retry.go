package solana

import (
	"context"
	"net/http"
	"time"

	ag_rpc "github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/jsonrpc"
)

// NewRetryingRPCClient builds an *ag_rpc.Client whose calls automatically
// retry on error. Devnet RPC (even via a paid Helius plan) rate-limits
// aggressively enough that a single-shot call routinely fails — this was
// observed hitting DetectTokenSource, BuildMeteoraSwapInstructions, and
// other read paths independently, each needing its own retry loop. A
// "account not found" response comes back from Solana RPC as a *successful*
// call with a nil Value, never an error, so retrying on any transport/RPC
// error here is safe — it can never turn a real miss into a false hit.
func NewRetryingRPCClient(rpcEndpoint string) *ag_rpc.Client {
	opts := &jsonrpc.RPCClientOpts{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
	inner := jsonrpc.NewClientWithOpts(rpcEndpoint, opts)
	return ag_rpc.NewWithCustomRPCClient(&retryingJSONRPCClient{inner: inner, attempts: 3})
}

type retryingJSONRPCClient struct {
	inner    ag_rpc.JSONRPCClient
	attempts int
}

func (r *retryingJSONRPCClient) withRetry(fn func() error) error {
	var lastErr error
	for i := 0; i < r.attempts; i++ {
		if err := fn(); err != nil {
			lastErr = err
			if i < r.attempts-1 {
				time.Sleep(time.Duration(300*(i+1)) * time.Millisecond)
			}
			continue
		}
		return nil
	}
	return lastErr
}

func (r *retryingJSONRPCClient) CallForInto(ctx context.Context, out interface{}, method string, params []interface{}) error {
	return r.withRetry(func() error {
		return r.inner.CallForInto(ctx, out, method, params)
	})
}

func (r *retryingJSONRPCClient) CallWithCallback(ctx context.Context, method string, params []interface{}, callback func(*http.Request, *http.Response) error) error {
	return r.withRetry(func() error {
		return r.inner.CallWithCallback(ctx, method, params, callback)
	})
}

func (r *retryingJSONRPCClient) CallBatch(ctx context.Context, requests jsonrpc.RPCRequests) (jsonrpc.RPCResponses, error) {
	var resp jsonrpc.RPCResponses
	err := r.withRetry(func() error {
		var e error
		resp, e = r.inner.CallBatch(ctx, requests)
		return e
	})
	return resp, err
}
