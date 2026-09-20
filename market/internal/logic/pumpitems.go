package logic

import (
	"context"
	"strconv"
	"strings"

	"dex/market/market"
	"dex/model"
	"dex/model/solmodel"
	"dex/pkg/chain"
	"gorm.io/gorm"
)

func meaningfulName(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && !strings.EqualFold(s, "token") && !strings.EqualFold(s, "unknown")
}

// Shared by requests and cache warming, so a background refresh cannot replace
// enriched metadata with placeholders or confuse the mint program with the DEX.
func LoadPumpTokenItems(ctx context.Context, db *gorm.DB, chainID int64, pairs []solmodel.Pair) ([]*market.PumpTokenItem, error) {
	items := make([]*market.PumpTokenItem, 0, len(pairs))
	if len(pairs) == 0 {
		return items, nil
	}
	addresses, pairAddresses := make([]string, 0, len(pairs)), make([]string, 0, len(pairs))
	for _, p := range pairs {
		addresses = append(addresses, p.TokenAddress)
		pairAddresses = append(pairAddresses, p.Address)
	}
	tokens, err := solmodel.NewTokenModel(db).FindAllByAddresses(ctx, chainID, addresses)
	if err != nil {
		return nil, err
	}
	tokenMap := make(map[string]*solmodel.Token, len(tokens))
	for i := range tokens {
		tokenMap[tokens[i].Address] = &tokens[i]
	}
	var assets []model.UserCreatedAsset
	if err := db.WithContext(ctx).Where("chain_id = ? AND asset_type = ? AND asset_address IN ?", chainID, "token", addresses).Find(&assets).Error; err != nil {
		return nil, err
	}
	assetMap := make(map[string]model.UserCreatedAsset, len(assets))
	for _, a := range assets {
		assetMap[a.AssetAddress] = a
	}
	stats, err := Fetch24hStats(ctx, db, chainID, pairAddresses)
	if err != nil {
		return nil, err
	}
	holders := FetchHolderCounts(ctx, db, chainID, addresses, tokenMap)
	for _, p := range pairs {
		item := buildPumpTokenItem(p, tokenMap[p.TokenAddress], assetMap[p.TokenAddress], stats[p.Address])
		item.HoldCount = holders[p.TokenAddress]
		items = append(items, item)
	}
	return items, nil
}

func buildPumpTokenItem(p solmodel.Pair, token *solmodel.Token, asset model.UserCreatedAsset, stats PumpToken24hStats) *market.PumpTokenItem {
	item := &market.PumpTokenItem{ChainId: p.ChainId, ChainIcon: chain.ChainId2ChainIcon(p.ChainId), TokenAddress: p.TokenAddress,
		PairAddress: p.Address, TokenName: p.TokenSymbol, TokenSymbol: p.TokenSymbol, MktCap: p.Fdv,
		Price: p.TokenPrice, DomesticProgress: p.PumpPoint, Program: p.Name, Txs_24H: stats.Txs, Vol_24H: stats.Vol}
	if !p.CreatedAt.IsZero() {
		item.LaunchTime = p.CreatedAt.Unix()
	} else if !p.BlockTime.IsZero() {
		item.LaunchTime = p.BlockTime.Unix()
	}
	var supply float64
	if token != nil {
		if meaningfulName(token.Name) {
			item.TokenName = token.Name
		}
		if meaningfulName(token.Symbol) {
			item.TokenSymbol = token.Symbol
		}
		item.TokenIcon, item.TwitterUsername, item.Telegram = token.Icon, token.TwitterUsername, token.Telegram
		supply = token.TotalSupply
	}
	if !meaningfulName(item.TokenName) && meaningfulName(asset.AssetName) {
		item.TokenName = asset.AssetName
	}
	if !meaningfulName(item.TokenSymbol) && meaningfulName(asset.AssetSymbol) {
		item.TokenSymbol = asset.AssetSymbol
	}
	// The indexed pair may only know the ticker, while Portfolio knows the name.
	if (token == nil || !meaningfulName(token.Name)) && meaningfulName(asset.AssetName) {
		item.TokenName = asset.AssetName
	}
	if supply <= 0 {
		supply, _ = strconv.ParseFloat(asset.TotalSupply, 64)
	}
	if stats.LastPrice > 0 {
		item.Price = stats.LastPrice
	}
	if stats.FirstPrice > 0 {
		item.Change24 = (stats.LastPrice/stats.FirstPrice - 1) * 100
	}
	if supply > 0 && item.Price > 0 {
		item.MktCap = supply * item.Price
	}
	return item
}
