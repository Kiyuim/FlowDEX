package solmodel

import (
	"context"
	"dex/pkg/constants"
	"dex/pkg/xcode"
	"fmt"

	. "github.com/klen-ygs/gorm-zero/gormc/sql"
	"gorm.io/gorm"
)

// avoid unused err
var _ = InitField
var _ PairModel = (*customPairModel)(nil)

type (
	// PairModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPairModel.
	PairModel interface {
		pairModel
		customPairLogicModel
	}

	customPairLogicModel interface {
		WithSession(tx *gorm.DB) PairModel
		FindOneByChainIdTokenAddress(ctx context.Context, chainId int64, tokenAddress string) (*Pair, error)
		FindLatestPumpLimit(ctx context.Context, pageNum, pageSize int32) ([]Pair, error)
		FindLatestCompletingPumpLimit(ctx context.Context, pageNum, pageSize int32) ([]Pair, error)
		FindLatestCompletePumpLimit(ctx context.Context, pageNum, pageSize int32) ([]Pair, error)
		FindSyncPump(ctx context.Context) ([]*Pair, error)
		FindOneByChainIdTokenAddressName(ctx context.Context, chainId int64, address string, name string) (*Pair, error)
	}

	customPairModel struct {
		*defaultPairModel
	}
)

func (c customPairModel) WithSession(tx *gorm.DB) PairModel {
	newModel := *c.defaultPairModel
	c.defaultPairModel = &newModel
	c.conn = tx
	return c
}

// NewPairModel returns a model for the database table.
func NewPairModel(conn *gorm.DB) PairModel {
	return &customPairModel{
		defaultPairModel: newPairModel(conn),
	}
}

func (m *defaultPairModel) customCacheKeys(data *Pair) []string {
	if data == nil {
		return []string{}
	}
	return []string{}
}

func (m customPairModel) FindOneByChainIdTokenAddressName(ctx context.Context, chainId int64, address string, name string) (*Pair, error) {
	var resp Pair
	err := m.conn.WithContext(ctx).Model(&Pair{}).Where("`chain_id` = ? and `token_address` = ? and `name` = ?", chainId, address, name).Take(&resp).Error
	return &resp, err
}

func (m customPairModel) FindSyncPump(ctx context.Context) ([]*Pair, error) {
	var resp []*Pair
	err := m.conn.WithContext(ctx).Model(&Pair{}).Where("name='PumpFun' and pump_status=1 and pump_point >= 0.8").Find(&resp).Error
	return resp, err
}

func (m customPairModel) FindOneByChainIdTokenAddress(ctx context.Context, chainId int64, tokenAddress string) (*Pair, error) {
	var res []Pair
	err := m.conn.WithContext(ctx).Model(&Pair{}).Where("chain_id = ? and token_address = ?", chainId, tokenAddress).Find(&res).Error
	if err != nil {
		return nil, err
	}

	if len(res) == 0 {
		return nil, xcode.NotingFoundError
	}

	var temporaryRes []Pair
	var contractResult Pair
	if len(res) > 0 {
		// Map to store pairs by category
		categoryMap := make(map[string][]Pair)
		// Group pairs by their name (category)
		for _, pair := range res {
			if pair.Name != "" {
				categoryMap[pair.Name] = append(categoryMap[pair.Name], pair)
			}
		}
		// Process categories (RaydiumV4, RaydiumConcentratedLiquidity)
		for _, pairs := range categoryMap {
			var LiquidityPair Pair
			var maxLiquidityCap float64

			// Find the pair with the largest market cap within each category
			for _, pair := range pairs {
				if pair.Liquidity > maxLiquidityCap {
					maxLiquidityCap = pair.Liquidity
					LiquidityPair = pair
				}
			}
			temporaryRes = append(temporaryRes, LiquidityPair)
		}
	}

	res = temporaryRes
	if len(res) == 1 {
		contractResult = res[0]
	} else if len(res) > 1 {
		//Get the pair with the highest market cap
		var maxLiquidityPair Pair
		var maxLiquidityCap float64
		for _, pair := range res {
			if pair.Name == "PumpFun" {
				continue
			}
			if pair.Liquidity > maxLiquidityCap {
				maxLiquidityCap = pair.Liquidity
				maxLiquidityPair = pair
			}
		}
		contractResult = maxLiquidityPair
	}

	return &contractResult, nil
}

func (m customPairModel) FindLatestPumpLimit(ctx context.Context, pageNum, pageSize int32) ([]Pair, error) {
	resp := make([]Pair, 0)
	err := m.conn.WithContext(ctx).Model(&Pair{}).Where("name IN ?", constants.BondingCurveSources).Order("block_num desc").Offset(int((pageNum - 1) * pageSize)).Limit(int(pageSize)).Find(&resp).Error
	return resp, err
}

func (m customPairModel) FindLatestCompletingPumpLimit(ctx context.Context, pageNum, pageSize int32) ([]Pair, error) {
	resp := make([]Pair, 0)
	err := m.conn.WithContext(ctx).Model(&Pair{}).Where("name IN ? and pump_status = 1", constants.BondingCurveSources).Order("created_at  desc").Offset(int((pageNum - 1) * pageSize)).Limit(int(pageSize)).Find(&resp).Error
	return resp, err
}

func (m customPairModel) FindLatestCompletePumpLimit(ctx context.Context, pageNum, pageSize int32) ([]Pair, error) {
	resp := make([]Pair, 0)
	err := m.conn.WithContext(ctx).Model(&Pair{}).Where("name IN ? and pump_status = 2", constants.BondingCurveSources).Order("block_num desc").Offset(int((pageNum - 1) * pageSize)).Limit(int(pageSize)).Find(&resp).Error
	fmt.Println("FindLatestCompletePumpLimit:", resp)
	return resp, err
}
