package block

import (
	"context"

	"github.com/duke-git/lancet/v2/slice"

	"dex/model/solmodel"
	"dex/pkg/types"

	"github.com/blocto/solana-go-sdk/client"
)

func (s *BlockService) SaveAccounts(ctx context.Context, trades []*types.TradeWithPair, block *client.Block) {
	var err error
	var balanceMap = make(map[string]int64)
	for _, tx := range block.Transactions {
		for i, balance := range tx.Meta.PostBalances {
			balanceMap[tx.AccountKeys[i].String()] = balance
		}
	}
	var balanceList []*solmodel.SolAccount
	for _, v := range trades {
		if v.Maker != "" {
			balanceList = append(balanceList, &solmodel.SolAccount{
				Address: v.Maker,
				Balance: balanceMap[v.Maker],
				Slot:    v.Slot,
			})
		}
		if v.To != "" && v.To != v.Maker {
			balanceList = append(balanceList, &solmodel.SolAccount{
				Address: v.To,
				Balance: balanceMap[v.To],
				Slot:    v.Slot,
			})
		}
	}

	// remove dup
	slice.Reverse(balanceList)
	balanceList = slice.UniqueByComparator[*solmodel.SolAccount](balanceList, func(item *solmodel.SolAccount, other *solmodel.SolAccount) bool {
		return item.Address == other.Address
	})
	slice.Reverse(balanceList)

	err = s.sc.SolAccountModel.BatchInsertAccounts(ctx, balanceList)
	if err != nil {
		s.Errorf("SaveAccounts:BatchInsertAccounts err:%v", err)
	}
}
