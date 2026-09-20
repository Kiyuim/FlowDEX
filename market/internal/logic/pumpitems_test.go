package logic

import (
	"dex/model"
	"dex/model/solmodel"
	"testing"
)

func TestPumpMetadataAndMarketCap(t *testing.T) {
	pair := solmodel.Pair{ChainId: 100000, TokenAddress: "mint", Address: "curve", TokenSymbol: "Token", Name: "PumpMeteora", Fdv: 5, TokenPrice: 0.001}
	token := &solmodel.Token{Name: "Token", Symbol: "TOKEN", TotalSupply: 1000000, Program: "SPL-Token"}
	asset := model.UserCreatedAsset{AssetName: "Real name", AssetSymbol: "REAL"}
	item := buildPumpTokenItem(pair, token, asset, PumpToken24hStats{Txs: 3, Vol: 45, FirstPrice: 0.001, LastPrice: 0.002})
	if item.TokenName != "Real name" || item.TokenSymbol != "REAL" || item.Program != "PumpMeteora" {
		t.Fatalf("metadata: %+v", item)
	}
	if item.MktCap != 2000 || item.Vol_24H != 45 || item.Change24 != 100 || item.Price != 0.002 {
		t.Fatalf("stats: %+v", item)
	}
}

func TestPumpWithoutRecentTradesKeepsPriceAndName(t *testing.T) {
	item := buildPumpTokenItem(solmodel.Pair{TokenPrice: 2}, &solmodel.Token{Name: "Indexed name", TotalSupply: 10}, model.UserCreatedAsset{}, PumpToken24hStats{})
	if item.TokenName != "Indexed name" || item.Price != 2 || item.MktCap != 20 || item.Vol_24H != 0 {
		t.Fatalf("item: %+v", item)
	}
}

func TestPumpCacheSeparatesPagesAndChains(t *testing.T) {
	a := PumpListCacheKey(100000, 1, 1, 50, "")
	for _, key := range []string{PumpListCacheKey(100000, 1, 2, 50, ""), PumpListCacheKey(1, 1, 1, 50, ""), PumpListCacheKey(100000, 1, 1, 50, "mint")} {
		if a == key {
			t.Fatal("cache key collision")
		}
	}
}
