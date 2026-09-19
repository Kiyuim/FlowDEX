package block

import (
	"dex/pkg/constants"
	"dex/pkg/types"
)

func (s *BlockService) FillTradeWithPairInfo(trade *types.TradeWithPair, slot int64) {
	trade.Slot = slot
	trade.BlockNum = slot
	trade.ChainIdInt = constants.SolChainIdInt
	trade.ChainId = constants.SolChainId
}
