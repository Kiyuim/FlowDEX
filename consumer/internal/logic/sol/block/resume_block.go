package block

import (
	"context"
	"strings"
	"time"

	"dex/consumer/internal/svc"
	"dex/model/solmodel"
	"dex/pkg/constants"
	"dex/pkg/types"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/threading"
)

type ResumeBlockService struct {
	BlockService
}

func (s *ResumeBlockService) Stop() {
	s.BlockService.Stop()
}

func (s *ResumeBlockService) Start() {
	s.GetBlockFromHttp()
}

func (s *ResumeBlockService) GetBlockFromHttp() {
	ctx := s.ctx
	for {
		select {
		case <-s.ctx.Done():
			return
		case slot, ok := <-s.slotChan:
			if !ok {
				return
			}
			threading.RunSafe(func() {
				s.ResumeBlock(ctx, int64(slot))
			})
		}
	}
}
func NewResumeBlockService(sc *svc.ServiceContext, name string, slotChan chan uint64, index int) *ResumeBlockService {
	service := NewBlockService(sc, name, slotChan, index)
	return &ResumeBlockService{
		BlockService: *service,
	}
}

// ResumeBlock 修复区块数据，只发送kafka
func (s *ResumeBlockService) ResumeBlock(ctx context.Context, slot int64) {

	beginTime := time.Now()

	s.Logger = s.Logger.WithFields(logx.Field("slot", slot))

	s.Infof("resumeBlock:%v start sol consumer will process block slot %v, queue size: %v", slot, slot, len(s.slotChan))

	if slot == 0 {
		s.Errorf("resumeBlock:%v slot is 0 %v, queue size: %v", slot, slot, len(s.slotChan))
		return
	}

	block := &solmodel.Block{
		Slot: slot,
	}

	blockInfo, err := GetSolBlockInfo(s.sc.GetSolClient(), ctx, uint64(slot))
	// metric count
	// xredis.IncrHeliusCallCount(ctx, s.sc.Redis, xredis.HeliusConsumerGetBlockInfo)
	if err != nil || blockInfo == nil {
		block.Status = constants.BlockFailed // failed
		// GetBlock err:{"code":-32007,"message":"Slot 311350484 was skipped, or missing due to ledger jump to recent snapshot","data":null}
		if err != nil && strings.Contains(err.Error(), "was skipped") {
			block.Status = constants.BlockSkipped
			s.Infof("resumeBlock:%v getSolBlockInfo was skipped, err: %v", slot, err)
			return
		}
		s.Errorf("resumeBlock:%v getSolBlockInfo error: %v", slot, err)
		return
	}
	blockTime := blockInfo.BlockTime.Format("2006-01-02 15:04:05")
	s.Infof("resumeBlock:%v getBlockInfo blockTime: %v,cur: %v, dur: %v, queue size: %v", slot, blockTime, time.Now().Format("15:04:05"), time.Since(beginTime), len(s.slotChan))

	var tokenAccountMap = make(map[string]*TokenAccount)
	solPrice := s.GetBlockSolPrice(ctx, blockInfo, tokenAccountMap)

	if solPrice == 0 {
		solPrice = s.solPrice
	}

	s.Infof("resumeBlock:%v getBlockSolPrice price: %v dur: %v", slot, solPrice, time.Since(beginTime))

	if solPrice == 0 {
		block.Status = constants.BlockFailed // failed
		s.Errorf("resumeBlock:%v sol price not found", slot)
		return
	}
	s.solPrice = solPrice

	block.BlockHeight = *(blockInfo.BlockHeight)
	block.BlockTime = *blockInfo.BlockTime
	block.Status = constants.BlockProcessed
	block.SolPrice = solPrice

	trades := make([]*types.TradeWithPair, 0, 1000)
	slice.ForEach[client.BlockTransaction](blockInfo.Transactions, func(index int, tx client.BlockTransaction) {
		decodeTx := &DecodedTx{
			BlockDb:         block,
			Tx:              &tx,
			TxIndex:         index,
			SolPrice:        solPrice,
			TokenAccountMap: tokenAccountMap,
		}
		trade, err := DecodeTx(ctx, s.sc, decodeTx)
		if err != nil {
			s.Errorf("resumeBlock:%v decodeTx err:%v, tx:%v", slot, err, decodeTx.TxHash)
			return
		}

		slice.ForEach(trade, func(_ int, item *types.TradeWithPair) {
			s.FillTradeWithPairInfo(item, slot)
		})

		trades = append(trades, trade...)
	})

	// remove dup
	s.Infof("resumeBlock:%v decodeTx sol tx_size before filter dup: %v, dur: %v, trade_size: %v", slot, len(blockInfo.Transactions), time.Since(beginTime), len(trades))

	trades = slice.UniqueByComparator[*types.TradeWithPair](trades, func(item *types.TradeWithPair, other *types.TradeWithPair) bool {
		return item.TxHash == other.TxHash
	})

	trades = slice.Filter[*types.TradeWithPair](trades, func(_ int, item *types.TradeWithPair) bool {
		if item.TokenPriceUSD == 0 {
			// s.Errorf("resumeBlock:%v trade.TokenPriceUSD is 0, trade: %#v", slot, item)
			return false
		}
		return true
	})

	s.Infof("resumeBlock:%v decodeTx sol tx_size  after filter dup: %v, dur: %v, trade_size: %v", slot, len(blockInfo.Transactions), time.Since(beginTime), len(trades))
	// s.Infof("resumeBlock:%v decodeTx sol tx_size: %v, dur: %v, trade_size: %v", slot, len(blockInfo.Transactions), time.Since(beginTime), len(trades))

	// s.SendTx(ctx, slot, trades)
	s.Infof("resumeBlock:%v sendTx tx_size: %v, dur: %v, trade_size: %v", slot, len(blockInfo.Transactions), time.Since(beginTime), len(trades))

}
