package cache

import (
	"dex/dataflow/internal/constants"
	"dex/dataflow/internal/svc"
	"sync"

	"github.com/zeromicro/go-zero/core/collection"
)

type KlineData struct {
	klines       *collection.Cache // pairAddr -> ring
	fluctuations *collection.Cache // pairAddr -> KlineFluctuation
	sc           *svc.ServiceContext
	limit        int64
	chainId      constants.ChainId
	interval     constants.KlineInterval
	loadFunc     LoadFunc
	lock         sync.RWMutex
}
