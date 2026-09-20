package logic

import (
	"context"
	"encoding/json"
	"fmt"

	"dex/market/internal/constants"
	"dex/market/internal/svc"
	"dex/market/market"
	"dex/model/solmodel"
	pkgconstants "dex/pkg/constants"
	"dex/pkg/solprice"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPumpTokenListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPumpTokenListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPumpTokenListLogic {
	return &GetPumpTokenListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PumpListCacheKey includes pagination and chain; old cache writers cannot
// overwrite a request with another page or with the pre-enrichment schema.
func PumpListCacheKey(chainID int64, status, page, size int32, mint string) string {
	return fmt.Sprintf("pump-token-list-v2:%d:%d:%d:%d:%s", chainID, status, page, size, mint)
}

func (l *GetPumpTokenListLogic) GetPumpTokenList(in *market.GetPumpTokenListRequest) (*market.GetPumpTokenListResponse, error) {
	page, size := in.PageNo, in.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 50
	}
	if size > 100 {
		size = 100
	}
	key := PumpListCacheKey(in.ChainId, in.PumpStatus, page, size, in.TokenAddress)
	if cached, err := l.svcCtx.RDS.Get(key); err == nil && cached != "" {
		var res market.GetPumpTokenListResponse
		if json.Unmarshal([]byte(cached), &res) == nil {
			return &res, nil
		}
	}
	query := l.svcCtx.DB.WithContext(l.ctx).Model(&solmodel.Pair{}).Where("chain_id = ?", in.ChainId)
	if in.TokenAddress != "" {
		query = query.Where("token_address = ?", in.TokenAddress)
	} else {
		query = query.Where("name IN ?", pkgconstants.BondingCurveSources)
		switch in.PumpStatus {
		case constants.PumpStatusNewCreation:
			query = query.Where("pump_status < 2 AND pump_point < ?", 0.8)
		case constants.PumpStatusCompleting:
			query = query.Where("pump_status < 2 AND pump_point >= ?", 0.8)
		case constants.PumpStatusCompleted:
			query = query.Where("pump_status >= 2")
		default:
			return nil, fmt.Errorf("invalid pump status: %d", in.PumpStatus)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var pairs []solmodel.Pair
	if err := query.Order("block_num DESC").Offset(int((page - 1) * size)).Limit(int(size)).Find(&pairs).Error; err != nil {
		return nil, err
	}
	// Preserve the existing diagnostic added locally before this change.
	for _, pair := range pairs {
		fmt.Println("PROBE_PAIR_NAME addr=", pair.Address, "name=", pair.Name, "tokenSymbol=", pair.TokenSymbol)
	}
	items, err := LoadPumpTokenItems(l.ctx, l.svcCtx.DB, in.ChainId, pairs)
	if err != nil {
		return nil, err
	}
	res := &market.GetPumpTokenListResponse{List: items, Total: int32(total), SolPriceUsd: solprice.GetSolUsdPrice()}
	if data, err := json.Marshal(res); err == nil {
		_ = l.svcCtx.RDS.Setex(key, string(data), 5)
	}
	return res, nil
}
