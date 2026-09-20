package ticker

import (
	"context"
	"dex/market/internal/logic"
	"dex/market/internal/svc"
	"dex/market/market"
	"dex/pkg/constants"
	"github.com/zeromicro/go-zero/core/logx"
	"sync"
	"time"
)

type PumpTicker struct {
	sc   *svc.ServiceContext
	done chan struct{}
	stop sync.Once
}

func NewPumpTicker(sc *svc.ServiceContext) *PumpTicker {
	return &PumpTicker{sc: sc, done: make(chan struct{})}
}
func (t *PumpTicker) Stop() { t.stop.Do(func() { close(t.done) }) }
func (t *PumpTicker) Start() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-t.done:
			return
		case <-ticker.C:
			for _, status := range []int32{1, 2, 4} {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				_, err := logic.NewGetPumpTokenListLogic(ctx, t.sc).GetPumpTokenList(&market.GetPumpTokenListRequest{
					ChainId: constants.SolChainIdInt, PumpStatus: status, PageNo: 1, PageSize: 50,
				})
				cancel()
				if err != nil {
					logx.Errorf("pump cache refresh: %v", err)
				}
			}
		}
	}
}
