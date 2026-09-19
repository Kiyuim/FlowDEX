package ticker

import (
	"context"
	"fmt"
	"time"

	"dex/consumer/internal/logic/pump"
	"dex/consumer/internal/svc"
	"dex/consumer/pkg/raydium"
	"dex/model/solmodel"

	pkgconstants "dex/pkg/constants"

	solclient "github.com/blocto/solana-go-sdk/client"
	"github.com/duke-git/lancet/v2/slice"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/threading"
)

var StartTime = time.Date(2024, time.December, 31, 0, 0, 0, 0, time.UTC)

type SolTicker struct {
	svc *svc.ServiceContext

	// client
	solClient *solclient.Client
	ctx       context.Context
	logger    logx.Logger
	timesMap  cmap.ConcurrentMap[string, int]
}

func NewSolTicker(ctx *svc.ServiceContext) *SolTicker {

	return &SolTicker{
		svc:       ctx,
		solClient: ctx.GetSolClient(),
		ctx:       context.Background(),
		logger:    logx.WithContext(context.Background()).WithFields(logx.Field("service", "sol-ticker")),
		timesMap:  cmap.New[int](),
	}
}

func (t *SolTicker) Start() {
	threading.GoSafe(func() {
		t.logger.Infof("solTicker startSyncPumpPoint start")
		for {
			err := t.startSyncPumpPoint()
			if err != nil {
				t.logger.Error(err.Error())
			}
			time.Sleep(1 * time.Minute)
		}
	})

}

func (t *SolTicker) Stop() {
}

func (t *SolTicker) startSyncPumpPoint() error {
	// Initialize trade order model
	model := t.svc.PairModel

	// Query orders waiting for market cap trigger
	pairs, err := model.FindSyncPump(t.ctx)
	if err != nil {
		return err
	}

	fmt.Println("startSyncPumpPoint pairs:", pairs)

	if len(pairs) == 0 {
		return nil
	}

	slice.ForEach(pairs, func(_ int, pair *solmodel.Pair) {
		time.Sleep(time.Millisecond * 50)
		PumpSwapPair, err := model.FindOneByChainIdTokenAddressName(t.ctx, pair.ChainId, pair.TokenAddress, pkgconstants.PumpSwap)
		if err != nil {
			fmt.Println("startSyncPumpPoint PumpSwapPair err:", err)
			return
		}
		if PumpSwapPair != nil {
			t.logger.Infof("startSyncPumpPoint Pump sync success: token address: %v", pair.TokenAddress)
			pair.PumpPoint = 1
			pair.PumpStatus = pump.PumpStatusMigrating
			_ = model.Update(t.ctx, pair)

			// update init token amount https://solscan.io/tx/28zCcUnHeHdK6m3TihMErVDTYjacyYiv5xFQvTvJozkYCBP17Xz5tocEcyPREFiBkgGkAYp75WjJ2UqdJEx12zSQ
			PumpSwapPair.InitTokenAmount = decimal.New(raydium.Pump2RaydiumInitTokenAmount, -int32(PumpSwapPair.TokenDecimal)).InexactFloat64()
			PumpSwapPair.InitBaseTokenAmount = decimal.New(raydium.Pump2RaydiumInitBaseTokenAmount, -int32(PumpSwapPair.BaseTokenDecimal)).InexactFloat64()
			_ = model.Update(t.ctx, PumpSwapPair)

			return
		}
	})
	return nil
}
