package block

import (
	"context"
	"dex/pkg/constants"
	"dex/pkg/raydium/cpmm/idl/generated/raydium_cp_swap"
	"dex/pkg/sol"
	"dex/pkg/transfer"
	"dex/pkg/types"
	"errors"
	"fmt"
	"strings"
	"time"

	"dex/model/solmodel"

	"dex/consumer/pkg/raydium"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/blocto/solana-go-sdk/rpc"
	set "github.com/duke-git/lancet/v2/datastructure/set"
	"github.com/duke-git/lancet/v2/slice"
	bin "github.com/gagliardetto/binary"
	"github.com/zeromicro/go-zero/core/threading"
)

func (s *BlockService) NewTradeModel(trade *types.TradeWithPair) (tradeDb *solmodel.Trade) {
	if trade == nil {
		s.Errorf("NewTradeModel: trade is nil, returning nil")
		return
	}

	s.Infof("NewTradeModel: Converting trade - TxHash: %s, PairAddr: %s, Type: %s",
		trade.TxHash, trade.PairAddr, trade.Type)

	now := time.Now()
	tradeDb = &solmodel.Trade{
		HashId:            trade.HashId,
		ChainId:           SolChainIdInt,
		PairAddr:          trade.PairAddr,
		TxHash:            trade.TxHash,
		Maker:             trade.Maker,
		TradeType:         trade.Type,
		BaseTokenAmount:   trade.BaseTokenAmount,
		TokenAmount:       trade.TokenAmount,
		BaseTokenPriceUsd: trade.BaseTokenPriceUSD,
		TotalUsd:          trade.TotalUSD,
		TokenPriceUsd:     trade.TokenPriceUSD,
		To:                trade.To,
		BlockNum:          trade.BlockNum,
		BlockTime:         time.Unix(trade.BlockTime, 0),
		BlockTimeStamp:    trade.BlockTime,
		SwapName:          trade.SwapName,

		CreatedAt: now,
		UpdatedAt: now,
	}

	s.Infof("NewTradeModel: Successfully created trade model - HashId: %s, ChainId: %d",
		tradeDb.HashId, tradeDb.ChainId)
	return
}

func (s *BlockService) SaveTrades(ctx context.Context, chainId int64, tradeMap map[string][]*types.TradeWithPair) {
	s.Infof("SaveTrades: Starting with %d pair addresses", len(tradeMap))

	group := threading.NewRoutineGroup()
	for key, trade := range tradeMap {
		s.Infof("SaveTrades: Processing pair %s with %d trades", key, len(trade))

		trade = slice.Filter[*types.TradeWithPair](trade, func(index int, item *types.TradeWithPair) bool {
			if item == nil {
				s.Infof("SaveTrades: Filtered out nil trade at index %d", index)
				return false
			}

			// Special handling for CLMM pool creation trades
			if item.SwapName == constants.RaydiumConcentratedLiquidity && item.Type == "create" {
				s.Infof("SaveTrades: Keeping CLMM pool creation trade at index %d", index)
				return true
			}

			if item.SwapName == constants.RaydiumConcentratedLiquidity && item.Type == "open_position" {
				s.Infof("SaveTrades: Keeping CLMM pool open_position trade at index %d", index)
				return true
			}

			// Normal filtering for buy/sell trades
			if item.Type != types.TradeTypeBuy && item.Type != types.TradeTypeSell {
				s.Infof("SaveTrades: Filtered out trade with invalid type %s at index %d", item.Type, index)
				return false
			}
			if item.TokenPriceUSD == 0 {
				s.Infof("SaveTrades: Filtered out trade with zero price at index %d", index)
				return false
			}
			return true
		})

		s.Infof("SaveTrades: After filtering, pair %s has %d valid trades", key, len(trade))

		group.RunSafe(func(key string, trade []*types.TradeWithPair) func() {
			return func() {
				txhashes := slice.Map[*types.TradeWithPair, string](trade, func(_ int, item *types.TradeWithPair) string {
					return item.TxHash
				})
				s.Infof("SaveTrades: will BatchSaveByTrade key: %v, tx hashes: %v", key, txhashes)

				fmt.Println("44444444444txhashes is:", trade)
				err := s.BatchSaveByTrade(ctx, chainId, key, trade)
				if err != nil {
					s.Errorf("SaveTrades: BatchSaveByTrade err:%v, key:%v, tx hashes: %v", err, key, txhashes)
				} else {
					s.Infof("SaveTrades: Successfully processed pair %s with %d trades", key, len(trade))
				}
			}
		}(key, trade))
	}
	s.Infof("SaveTrades: Waiting for all goroutines to complete")
	group.Wait()
	s.Infof("SaveTrades: All trades processed successfully")
}

func (s *BlockService) BatchSaveByTrade(ctx context.Context, chainId int64, pairAddress string, trades []*types.TradeWithPair) (err error) {
	if err = s.SavePairInfo(ctx, chainId, pairAddress, trades); err != nil {
		s.Error(fmt.Errorf("batchSaveByTrade:savePairInfo err:%v", err))
	}
	if err = s.BatchSaveTrade(ctx, trades); err != nil {
		s.Error(fmt.Errorf("batchSaveByTrade:saveTrade err:%w", err))
	}
	return
}

func (s *BlockService) SavePairInfo(ctx context.Context, chainId int64, pairAddress string, trades []*types.TradeWithPair) (err error) {
	fmt.Println("11111111111SavePairInfo: 开始保存pair信息")

	fmt.Println("22222222222trades is:", len(trades))

	if trades == nil {
		fmt.Println("33333333333trades is:", trades[0].TxHash)
		return
	}

	// fmt.Println("33333333333trades is:", trades)

	if len(trades) == 0 {
		fmt.Println("111111111SavePairInfo: trades is empty, returning early")
		return nil
	}

	trade := trades[len(trades)-1]
	var tokenDb *solmodel.Token
	tokenDb, err = s.SaveToken(ctx, trade)
	if err != nil || tokenDb == nil {
		fmt.Println("111111111SavePairInfo:SaveToken err:", err)
		s.Error("SavePairInfo:SaveToken err:", err)
		return err
	}

	if tokenDb.TotalSupply == 0 {
		fmt.Printf("111111111SavePairInfo token totalSupply is 0, tokenDb: %#v", tokenDb)
		s.Errorf("savePairInfo token totalSupply is 0, tokenDb: %#v", tokenDb)
	}
	fmt.Println("5555555555555555555555")

	// fill data
	for _, tradeInfo := range trades {
		tradeInfo.PairInfo.TokenTotalSupply = tokenDb.TotalSupply
	}

	fmt.Println("pair list is:", trade.PairInfo)
	fmt.Println("888888888888888888888888is:", trade.ClmmPoolInfoV1)

	_, err = s.SavePair(ctx, trade, tokenDb)
	if err != nil {
		fmt.Printf("77777777777777777777:SavePair err: %v", err)
	}

	for _, tradeInfo := range trades {
		tradeInfo.Mcap = trade.Mcap
		tradeInfo.Fdv = trade.Fdv
	}
	fmt.Println("6666666666666666666666")

	return
}

func (s *BlockService) BatchSaveTrade(ctx context.Context, trades []*types.TradeWithPair) error {
	s.Infof("BatchSaveTrade: Starting with %d trades", len(trades))

	if trades == nil {
		s.Infof("BatchSaveTrade: trades is nil, returning early")
		return nil
	}

	if len(trades) == 0 {
		s.Infof("BatchSaveTrade: trades slice is empty, returning early")
		return nil
	}

	// Log some trade details for debugging
	for i, trade := range trades {
		if i < 3 { // Only log first 3 trades to avoid spam
			s.Infof("BatchSaveTrade: Trade[%d] - TxHash: %s, PairAddr: %s, Type: %s, TokenAmount: %f",
				i, trade.TxHash, trade.PairAddr, trade.Type, trade.TokenAmount)
		}
	}

	s.Infof("BatchSaveTrade: Converting trades to database models")
	tradeDbs := slice.Map[*types.TradeWithPair, *solmodel.Trade](trades, func(_ int, trade *types.TradeWithPair) *solmodel.Trade {
		return s.NewTradeModel(trade)
	})

	s.Infof("BatchSaveTrade: Converted %d trades to database models", len(tradeDbs))

	// Log some database model details
	for i, tradeDb := range tradeDbs {
		if i < 3 { // Only log first 3 to avoid spam
			s.Infof("BatchSaveTrade: TradeDb[%d] - TxHash: %s, PairAddr: %s, TradeType: %s",
				i, tradeDb.TxHash, tradeDb.PairAddr, tradeDb.TradeType)
		}
	}

	s.Infof("BatchSaveTrade: Calling BatchInsertTrades with %d trades", len(tradeDbs))
	err := s.sc.TradeModel.BatchInsertTrades(ctx, tradeDbs)
	if err != nil {
		s.Errorf("BatchSaveTrade: BatchInsertTrades failed with error: %v", err)
		return err
	}

	s.Infof("BatchSaveTrade: Successfully inserted %d trades", len(tradeDbs))
	return nil
}

func (s *BlockService) UpdateTokenMints(ctx context.Context, tokenMints []*types.TradeWithPair) {
	client := s.sc.GetSolClient()

	hashSet := set.New[string]()

	slice.ForEach(tokenMints, func(_ int, item *types.TradeWithPair) {
		if item != nil && item.Type == types.TradeTokenMint {
			mintTo := item.InstructionMintTo
			if hashSet.Contain(mintTo.Mint.String()) {
				return
			}
			token, err := s.sc.TokenModel.FindOneByChainIdAddress(s.ctx, int64(item.ChainIdInt), mintTo.Mint.String())
			if err == nil && token != nil {
				totalSupply, err := sol.GetTokenTotalSupply(client, s.ctx, mintTo.Mint.String())
				if err == nil && totalSupply.IsPositive() {
					token.TotalSupply = totalSupply.InexactFloat64()
					s.Infof("UpdateTokenMints: update totalSupply, token address: %v, total supply: %v, tx hash: %v", token.Address, token.TotalSupply, item.TxHash)
					hashSet.Add(mintTo.Mint.String())
					_ = s.sc.TokenModel.Update(ctx, token)
				}
			}
		}
	})
}

func (s *BlockService) UpdateTokenBurns(ctx context.Context, tokenBurns []*types.TradeWithPair) {
	client := s.sc.GetSolClient()

	hashSet := set.New[string]()

	slice.ForEach(tokenBurns, func(_ int, item *types.TradeWithPair) {
		if item == nil {
			return
		}
		if item != nil && (item.Type == types.TradeTokenBurn || item.Type == "token_burn") {
			burn := item.InstructionBurn
			if hashSet.Contain(burn.Mint.String()) {
				return
			}
			token, err := s.sc.TokenModel.FindOneByChainIdAddress(s.ctx, int64(item.ChainIdInt), burn.Mint.String())
			if err == nil && token != nil {
				totalSupply, err := sol.GetTokenTotalSupply(client, s.ctx, burn.Mint.String())
				if err == nil && totalSupply.IsPositive() {
					token.TotalSupply = totalSupply.InexactFloat64()
					s.Infof("UpdateTokenBurns: update totalSupply, token address: %v, total supply: %v, tx hash: %v", token.Address, token.TotalSupply, item.TxHash)
					hashSet.Add(burn.Mint.String())
					_ = s.sc.TokenModel.Update(ctx, token)
				}
			}
		}
	})
}

func (s *BlockService) SaveRaydiumCLMMPoolInfo(ctx context.Context, pair *types.TradeWithPair) (err error) {
	// Add panic recovery
	defer func() {
		if r := recover(); r != nil {
			var txHash string
			if pair != nil {
				txHash = pair.TxHash
			} else {
				txHash = "unknown"
			}
			s.Errorf("SaveRaydiumCLMMPoolInfo: panic occurred: %v, tx hash: %v", r, txHash)
			err = fmt.Errorf("SaveRaydiumCLMMPoolInfo: panic occurred: %v", r)
		}
	}()

	s.Infof("Starting SaveRaydiumCLMMPoolInfo for txHash: %s, pair address: %s, swapName: %s", pair.TxHash, pair.PairAddr, pair.SwapName)

	if pair.SwapName != constants.RaydiumConcentratedLiquidity {
		s.Infof("SaveRaydiumCLMMPoolInfo: Skipping - not a CLMM pool. SwapName: %s", pair.SwapName)
		return nil
	}

	if pair.ClmmPoolInfoV1 == nil && pair.ClmmPoolInfoV2 == nil {
		s.Infof("SaveRaydiumCLMMPoolInfo: v1 and v2 are nil, Skipping - no CLMM pool info. SwapName: %s", pair.SwapName)
		return nil
	}
	if pair.ClmmPoolInfoV1 == nil {
		s.Infof("SaveRaydiumCLMMPoolInfo: v1 is nil, Skipping - no CLMM pool info. SwapName: %s", pair.SwapName)
		return nil
	}

	if pair.ClmmPoolInfoV1 != nil {
		fmt.Println("pair.ClmmPoolInfoV1 is:", pair.ClmmPoolInfoV1)
		// Get pool state string safely
		var poolStateStr string
		func() {
			defer func() {
				if r := recover(); r != nil {
					s.Errorf("SaveRaydiumCLMMPoolInfo: panic getting poolState string: %v, tx hash: %v", r, pair.TxHash)
					poolStateStr = ""
				}
			}()
			poolStateStr = pair.ClmmPoolInfoV1.PoolState.PubKey.String()
		}()

		if poolStateStr == "" {
			return fmt.Errorf("SaveRaydiumCLMMPoolInfo: failed to get pool state string")
		}

		// Process remaining accounts safely
		var remainingAccountsStr string
		func() {
			defer func() {
				if r := recover(); r != nil {
					s.Errorf("SaveRaydiumCLMMPoolInfo: panic processing remaining accounts: %v, tx hash: %v", r, pair.TxHash)
					remainingAccountsStr = "[]"
				}
			}()

			remainingAccounts := make([]string, 0)
			for _, account := range pair.ClmmPoolInfoV1.RemainingAccounts {
				remainingAccounts = append(remainingAccounts, account.PubKey.String())
			}
			remainingAccountsStr, _ = transfer.Struct2String(remainingAccounts)
		}()

		now := time.Now()

		// Find existing pool
		dbPool, err := s.sc.SolRaydiumCLMMPoolV1Model.FindOneByPoolState(ctx, poolStateStr)

		fmt.Println("pair.ClmmPoolInfoV1.TickArray is:", pair.ClmmPoolInfoV1.TickArray)
		if pair.ClmmPoolInfoV1 != nil && dbPool != nil {
			dbPool.TickArray = pair.ClmmPoolInfoV1.TickArray.String()
		}

		// Remove old logic using strings.Split and strings.Join
		// remainingAccounts := strings.Split(dbPool.RemainingAccounts, ",")
		// for _, account := range pair.ClmmPoolInfoV1.RemainingAccounts {
		//     accountStr, _ := transfer.Struct2String(account)
		//     remainingAccounts = append(remainingAccounts, accountStr)
		// }
		// dbPool.RemainingAccounts = strings.Join(remainingAccounts, ",")

		// Instead, always use the JSON array string
		if dbPool != nil {
			dbPool.RemainingAccounts = remainingAccountsStr

			fmt.Println("remainingAccountsStr is:", dbPool.RemainingAccounts)
			fmt.Println("dbPool.TickArray is:", dbPool.TickArray)

		}

		switch {
		case err == nil:
			// Update existing pool
			if dbPool != nil {
				dbPool.UpdatedAt = now
				func() {
					defer func() {
						if r := recover(); r != nil {
							s.Errorf("SaveRaydiumCLMMPoolInfo: panic in Update: %v, tx hash: %v", r, pair.TxHash)
						}
					}()
					_ = s.sc.SolRaydiumCLMMPoolV1Model.Update(ctx, dbPool)
				}()
				s.Infof("SaveRaydiumCLMMPoolInfo:FindOneByPoolState v1 update success, id: %v, hash: %v", poolStateStr, pair.TxHash)
			}
			return nil
		case errors.Is(err, solmodel.ErrNotFound) || strings.Contains(err.Error(), "record not found") || err.Error() == "sql: no rows in result set":
			// Create new pool record
			var ammConfigStr, inputVaultStr, outputVaultStr, observationStateStr, tokenProgramStr, tokenProgram2022Str, memoProgramStr, inputVaultMintStr, outputVaultMintStr, tickArrayStr string

			// Safely get all the PubKey strings
			func() {
				defer func() {
					if r := recover(); r != nil {
						s.Errorf("SaveRaydiumCLMMPoolInfo: panic getting ClmmPoolInfoV1 PubKey strings: %v, tx hash: %v", r, pair.TxHash)
					}
				}()
				ammConfigStr = pair.ClmmPoolInfoV1.AmmConfig.String()
				inputVaultStr = pair.ClmmPoolInfoV1.InputVault.PubKey.String()
				outputVaultStr = pair.ClmmPoolInfoV1.OutputVault.PubKey.String()
				observationStateStr = pair.ClmmPoolInfoV1.ObservationState.PubKey.String()
				tokenProgramStr = pair.ClmmPoolInfoV1.TokenProgram.String()
				tokenProgram2022Str = pair.ClmmPoolInfoV1.TokenProgram2022.String()
				memoProgramStr = pair.ClmmPoolInfoV1.MemoProgram.String()
				inputVaultMintStr = pair.ClmmPoolInfoV1.InputVaultMint.String()
				outputVaultMintStr = pair.ClmmPoolInfoV1.OutputVaultMint.String()
				tickArrayStr = pair.ClmmPoolInfoV1.TickArray.String()

				s.Infof("SaveRaydiumCLMMPoolInfo: ClmmPoolInfoV1 details - InputVaultMint: %s, OutputVaultMint: %s, PoolState: %s",
					inputVaultMintStr, outputVaultMintStr, poolStateStr)
			}()

			info := &solmodel.ClmmPoolInfoV1{
				AmmConfig:         ammConfigStr,
				PoolState:         poolStateStr,
				InputVault:        inputVaultStr,
				OutputVault:       outputVaultStr,
				ObservationState:  observationStateStr,
				TokenProgram:      tokenProgramStr,
				TokenProgram2022:  tokenProgram2022Str,
				MemoProgram:       memoProgramStr,
				InputVaultMint:    inputVaultMintStr,
				OutputVaultMint:   outputVaultMintStr,
				TickArray:         tickArrayStr,
				RemainingAccounts: remainingAccountsStr,
				TxHash:            pair.TxHash,
				TradeFeeRate:      int64(pair.ClmmPoolInfoV1.TradeFeeRate),
				CreatedAt:         now,
				UpdatedAt:         now,
			}

			// Safely insert record
			func() {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("SaveRaydiumCLMMPoolInfo: panic in Insert: %v", r)
					}
				}()
				err = s.sc.SolRaydiumCLMMPoolV1Model.Insert(ctx, info)
			}()

			if err != nil {
				if !strings.Contains(err.Error(), "Duplicate entry") {
					s.Errorf("SaveRaydiumCLMMPoolInfo: Failed to insert CLMM V1 pool: %v, txHash: %s, poolState: %s",
						err, pair.TxHash, poolStateStr)
					return fmt.Errorf("SaveRaydiumCLMMPoolInfo:SolRaydiumCLMMPoolV1Model.Insert %#v err:%v", info, err)
				}
				s.Infof("SaveRaydiumCLMMPoolInfo: Duplicate CLMM V1 pool entry, skipping: %s", poolStateStr)
				return nil
			}
			s.Infof("SaveRaydiumCLMMPoolInfo: Successfully inserted CLMM V1 pool: %s, txHash: %s, InputMint: %s, OutputMint: %s",
				poolStateStr, pair.TxHash, info.InputVaultMint, info.OutputVaultMint)
		default:
			s.Errorf("SaveRaydiumCLMMPoolInfo: Failed to find CLMM V1 pool: %v, txHash: %s, poolState: %s",
				err, pair.TxHash, poolStateStr)
			return fmt.Errorf("SaveRaydiumCLMMPoolInfo:SolRaydiumCLMMPoolV1Model.FindOneByPoolState err:%w", err)
		}
	}

	if pair.ClmmPoolInfoV2 != nil {
		// Safely print ClmmPoolInfoV2 without causing panics
		func() {
			defer func() {
				if r := recover(); r != nil {
					s.Errorf("SaveRaydiumCLMMPoolInfo: panic printing ClmmPoolInfoV2: %v, tx hash: %v", r, pair.TxHash)
				}
			}()
			s.Infof("SaveRaydiumCLMMPoolInfo: Processing CLMM V2 pool for txHash: %s", pair.TxHash)
		}()

		// Get pool state string safely
		var poolStateStr string
		func() {
			defer func() {
				if r := recover(); r != nil {
					s.Errorf("SaveRaydiumCLMMPoolInfo: panic getting poolState string: %v, tx hash: %v", r, pair.TxHash)
					poolStateStr = ""
				}
			}()
			poolStateStr = pair.ClmmPoolInfoV2.PoolState.PubKey.String()
		}()

		if poolStateStr == "" {
			s.Errorf("SaveRaydiumCLMMPoolInfo: Failed - empty poolState for CLMM V2, txHash: %s", pair.TxHash)
			return fmt.Errorf("SaveRaydiumCLMMPoolInfo: failed to get pool state string")
		}

		// Process remaining accounts safely
		var remainingAccountsStr string
		func() {
			defer func() {
				if r := recover(); r != nil {
					s.Errorf("SaveRaydiumCLMMPoolInfo: panic processing remaining accounts: %v, tx hash: %v", r, pair.TxHash)
					remainingAccountsStr = "[]"
				}
			}()

			remainingAccounts := make([]string, 0)
			for _, account := range pair.ClmmPoolInfoV2.RemainingAccounts {
				remainingAccounts = append(remainingAccounts, account.PubKey.String())
			}
			remainingAccountsStr, _ = transfer.Struct2String(remainingAccounts)
		}()

		now := time.Now()

		// Find existing pool
		dbPool, err := s.sc.SolRaydiumCLMMPoolV2Model.FindOneByPoolState(ctx, poolStateStr)

		switch {
		case err == nil:
			// Update existing pool
			if dbPool != nil {
				dbPool.UpdatedAt = now
				func() {
					defer func() {
						if r := recover(); r != nil {
							s.Errorf("SaveRaydiumCLMMPoolInfo: panic in Update: %v, tx hash: %v", r, pair.TxHash)
						}
					}()
					_ = s.sc.SolRaydiumCLMMPoolV2Model.Update(ctx, dbPool)
				}()
				s.Infof("SaveRaydiumCLMMPoolInfo:FindOneByPoolState v2 update success, id: %v, hash: %v", poolStateStr, pair.TxHash)
			}
			return nil
		case errors.Is(err, solmodel.ErrNotFound) || strings.Contains(err.Error(), "record not found") || err.Error() == "sql: no rows in result set":
			// Create new pool record
			var ammConfigStr, inputVaultStr, outputVaultStr, observationStateStr, tokenProgramStr, tokenProgram2022Str, memoProgramStr, inputVaultMintStr, outputVaultMintStr string

			// Safely get all the PubKey strings
			func() {
				defer func() {
					if r := recover(); r != nil {
						s.Errorf("SaveRaydiumCLMMPoolInfo: panic getting ClmmPoolInfoV2 PubKey strings: %v, tx hash: %v", r, pair.TxHash)
					}
				}()
				ammConfigStr = pair.ClmmPoolInfoV2.AmmConfig.String()
				inputVaultStr = pair.ClmmPoolInfoV2.InputVault.PubKey.String()
				outputVaultStr = pair.ClmmPoolInfoV2.OutputVault.PubKey.String()
				observationStateStr = pair.ClmmPoolInfoV2.ObservationState.PubKey.String()
				tokenProgramStr = pair.ClmmPoolInfoV2.TokenProgram.String()
				tokenProgram2022Str = pair.ClmmPoolInfoV2.TokenProgram2022.String()
				memoProgramStr = pair.ClmmPoolInfoV2.MemoProgram.String()
				inputVaultMintStr = pair.ClmmPoolInfoV2.InputVaultMint.String()
				outputVaultMintStr = pair.ClmmPoolInfoV2.OutputVaultMint.String()

				s.Infof("SaveRaydiumCLMMPoolInfo: ClmmPoolInfoV2 details - InputVaultMint: %s, OutputVaultMint: %s, PoolState: %s",
					inputVaultMintStr, outputVaultMintStr, poolStateStr)
			}()

			info := &solmodel.ClmmPoolInfoV2{
				AmmConfig:         ammConfigStr,
				PoolState:         poolStateStr,
				InputVault:        inputVaultStr,
				OutputVault:       outputVaultStr,
				ObservationState:  observationStateStr,
				TokenProgram:      tokenProgramStr,
				TokenProgram2022:  tokenProgram2022Str,
				MemoProgram:       memoProgramStr,
				InputVaultMint:    inputVaultMintStr,
				OutputVaultMint:   outputVaultMintStr,
				RemainingAccounts: remainingAccountsStr,
				TxHash:            pair.TxHash,
				TradeFeeRate:      int64(pair.ClmmPoolInfoV2.TradeFeeRate),
				CreatedAt:         now,
				UpdatedAt:         now,
			}

			// Safely insert record
			func() {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("SaveRaydiumCLMMPoolInfo: panic in Insert: %v", r)
					}
				}()
				err = s.sc.SolRaydiumCLMMPoolV2Model.Insert(ctx, info)
			}()

			if err != nil {
				if !strings.Contains(err.Error(), "Duplicate entry") {
					s.Errorf("SaveRaydiumCLMMPoolInfo: Failed to insert CLMM V2 pool: %v, txHash: %s, poolState: %s",
						err, pair.TxHash, poolStateStr)
					return fmt.Errorf("SaveRaydiumCLMMPoolInfo:SolRaydiumCLMMPoolV2Model.Insert %#v err:%v", info, err)
				}
				s.Infof("SaveRaydiumCLMMPoolInfo: Duplicate CLMM V2 pool entry, skipping: %s", poolStateStr)
				return nil
			}
			s.Infof("SaveRaydiumCLMMPoolInfo: Successfully inserted CLMM V2 pool: %s, txHash: %s, InputMint: %s, OutputMint: %s",
				poolStateStr, pair.TxHash, info.InputVaultMint, info.OutputVaultMint)
		default:
			s.Errorf("SaveRaydiumCLMMPoolInfo: Failed to find CLMM V2 pool: %v, txHash: %s, poolState: %s",
				err, pair.TxHash, poolStateStr)
			return fmt.Errorf("SaveRaydiumCLMMPoolInfo:SolRaydiumCLMMPoolV2Model.FindOneByPoolState err:%w", err)
		}
	}

	return nil
}

func (s *BlockService) SaveRaydiumCPMMPoolInfo(ctx context.Context, pair *types.TradeWithPair) (err error) {

	if pair.SwapName != constants.RaydiumCPMM {
		return nil
	}

	if pair.CpmmPoolInfo != nil {

		now := time.Now()

		_, err := s.sc.SolRaydiumCPMMPoolModel.FindOneByPoolState(ctx, pair.CpmmPoolInfo.PoolState)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, solmodel.ErrNotFound):

			pair.CpmmPoolInfo.CreatedAt = now
			pair.CpmmPoolInfo.UpdatedAt = now

			// parse fee
			solClient := s.sc.GetSolClient()
			accountInfo, err := solClient.GetAccountInfoWithConfig(ctx, pair.CpmmPoolInfo.AmmConfig, client.GetAccountInfoConfig{
				Commitment: rpc.CommitmentConfirmed,
			})
			if err != nil {
				return err
			}
			ammConfig := raydium_cp_swap.AmmConfig{}
			if err := ammConfig.UnmarshalWithDecoder(bin.NewBorshDecoder(accountInfo.Data)); err != nil {
				return err
			}

			pair.CpmmPoolInfo.TradeFeeRate = int64(ammConfig.TradeFeeRate)

			if err = s.sc.SolRaydiumCPMMPoolModel.Insert(ctx, pair.CpmmPoolInfo); err != nil {
				if !strings.Contains(err.Error(), "Duplicate entry") {
					err = fmt.Errorf("SaveRaydiumCPMMPoolInfo:SolRaydiumCPMMPoolModel.Insert %#v err:%v", pair.CpmmPoolInfo, err)
					return err
				} else {
					return nil
				}
			}
			s.Infof("SaveRaydiumCPMMPoolInfo:SolRaydiumCPMMPoolModel.Insert success:%#v", pair.CpmmPoolInfo)
		default:
			err = fmt.Errorf("SaveRaydiumCPMMPoolInfo:SolRaydiumCPMMPoolModel.FindOneByAmmIdChainId err:%w", err)
		}
	}

	return
}

func (s *BlockService) SavePumpSwapPoolInfo(ctx context.Context, pair *types.TradeWithPair) (err error) {
	if pair.SwapName != constants.PumpSwap && pair.SwapName != "PumpSwap" {
		return nil
	}

	if pair.PumpAmmInfo != nil {
		_, err := s.sc.PumpAmmInfoModel.FindOneByPoolAccount(ctx, pair.PumpAmmInfo.PoolAccount)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, solmodel.ErrNotFound) || err.Error() == "record not found":
			if err = s.sc.PumpAmmInfoModel.Insert(ctx, pair.PumpAmmInfo); err != nil {
				if !strings.Contains(err.Error(), "Duplicate entry") {
					err = fmt.Errorf("SaveRaydiumCPMMPoolInfo:SolRaydiumCPMMPoolModel.Insert %#v err:%v", pair.CpmmPoolInfo, err)
					return err
				} else {
					return nil
				}
			}
		default:
			err = fmt.Errorf("SavePumpSwapPoolInfo:777 PumpAmmInfoModel.FindOneByPoolAccount err:%w", err)
		}
	}

	return
}

func (s *BlockService) BatchSaveRaydiumPool(ctx context.Context, poolMap map[string]*solmodel.RaydiumPool) {
	// var poolMap = make(map[string]*solmodel.RaydiumPool)
	// for _, trade := range trades {
	// 	if trade.RaydiumPool != nil {
	// 		// if err := types.ValidateRaydiumPool(trade.RaydiumPool); err != nil {
	// 		// 	// s.Errorf("validateRaydiumPool %#v err:%v", trade.RaydiumPool, err)
	// 		// 	continue
	// 		// }
	// 		poolMap[trade.RaydiumPool.AmmId] = trade.RaydiumPool
	// 	}
	// }
	//
	group := threading.NewRoutineGroup()
	for _, pool := range poolMap {
		group.RunSafe(func(pool *solmodel.RaydiumPool) func() {
			return func() {
				err := s.SaveRaydiumPool(ctx, pool)
				if err != nil {
					s.Errorf("BatchSaveRaydiumPool err:%v, pool: %#v", err, pool)
					return
				}
			}
		}(pool))
	}
	group.Wait()
	return
}

func (s *BlockService) SaveRaydiumPool(ctx context.Context, raydiumPool *solmodel.RaydiumPool) (err error) {
	if raydiumPool == nil {
		return errors.New("saveRaydiumPool raydiumPool is nil")
	}

	dbRecord, err := s.sc.SolRaydiumPoolModel.FindOneByChainIdAmmId(ctx, raydiumPool.ChainId, raydiumPool.AmmId)
	switch {
	case err == nil:
		if len(dbRecord.QuoteMint) == 0 || len(dbRecord.BaseMint) == 0 {
			err = s.fillRaydiumPoolData(raydiumPool)
			if err != nil {
				s.Errorf("SaveRaydiumPool:fillRaydiumPoolData %#v, err:%v", raydiumPool, err)
				// return err
			}
			dbRecord.QuoteMint = raydiumPool.QuoteMint
			dbRecord.BaseMint = raydiumPool.BaseMint
			err := s.sc.SolRaydiumPoolModel.Update(ctx, dbRecord)
			if err != nil {
				s.Errorf("SaveRaydiumPool:raydiumPoolModel.Update err:%v", err)
				return fmt.Errorf("SaveRaydiumPool:raydiumPoolModel update err:%w", err)
			}
			s.Infof("SaveRaydiumPool:raydiumPoolModel.Update success:%#v", dbRecord)
		}
		return nil

	case errors.Is(err, solmodel.ErrNotFound):
		_ = s.fillRaydiumPoolData(raydiumPool)
		// double check
		if len(raydiumPool.AmmTargetOrders) == 0 {
			return fmt.Errorf("SaveRaydiumPool:raydiumPoolModel.Insert err:%v", err)
		}
		if err := types.ValidateRaydiumPool(raydiumPool); err != nil {
			// s.Errorf("SaveRaydiumPool:validateRaydiumPool insert %#v err:%v", raydiumPool, err)
			err = fmt.Errorf("SaveRaydiumPool:validateRaydiumPool insert %#v err:%v", raydiumPool, err)
			return err
		}
		err = s.sc.SolRaydiumPoolModel.Insert(ctx, raydiumPool)
		if err != nil {
			if !strings.Contains(err.Error(), "Duplicate entry") {
				err = fmt.Errorf("SaveRaydiumPool:raydiumPoolModel.Insert %#v err:%v", raydiumPool, err)
				return
			} else {
				return nil
			}

		}
		s.Infof("SaveRaydiumPool:raydiumPoolModel.Insert success:%#v", raydiumPool)
	default:
		err = fmt.Errorf("SaveRaydiumPool:raydiumPoolModel.FindOneByAmmIdChainId err:%w", err)
	}
	return
}

func (s *BlockService) fillRaydiumPoolData(raydiumPool *solmodel.RaydiumPool) error {
	if len(raydiumPool.AmmTargetOrders) == 0 || len(raydiumPool.BaseMint) == 0 || len(raydiumPool.QuoteMint) == 0 {
		info, err := raydium.GetAmmPoolInfo(s.sc.GetSolClient(), s.ctx, raydiumPool.AmmId)
		if err != nil {
			s.Errorf("fillRaydiumPoolData:GetAmmPoolInfo err:%v, ammId: %v", err, raydiumPool.AmmId)
			return err
		}
		raydiumPool.AmmTargetOrders = info.TargetOrders.String()
		raydiumPool.BaseMint = info.CoinVaultMint.String()
		raydiumPool.QuoteMint = info.PcVaultMint.String()
		s.Infof("fillRaydiumPoolData: amm id: %v, BaseMint: %v, QuoteMint: %v", raydiumPool.AmmId, raydiumPool.BaseMint, raydiumPool.QuoteMint)
	}
	return nil
}

func (s *BlockService) SendTokenPrice2TradeRPC(ctx context.Context, solPrice float64, tradeMap map[string][]*types.TradeWithPair) {
	group := threading.NewRoutineGroup()

	for _, tradeSlice := range tradeMap {
		if tradeSlice == nil || len(tradeSlice) == 0 {
			continue
		}

		group.RunSafe(func(solPrice float64, lastTrade *types.TradeWithPair) func() {
			return func() {
				s.sendTokenPrice2Trade(ctx, solPrice, lastTrade)
			}
		}(solPrice, tradeSlice[len(tradeSlice)-1]))
	}
	group.Wait()
}

func (s *BlockService) SaveTokenAccounts(ctx context.Context, trades []*types.TradeWithPair, tokenAccountMap map[string]*TokenAccount) {
	var tokenAccounts []*solmodel.SolTokenAccount

	for _, tokenAccount := range tokenAccountMap {

		status := 0
		if tokenAccount.Closed {
			status = 1
		}

		if tokenAccount.TokenAddress == constants.TokenStrWrapSol {
			continue
		}
		solTokenAccount := &solmodel.SolTokenAccount{
			OwnerAddress:        tokenAccount.Owner,
			Status:              int64(status),
			ChainId:             SolChainIdInt,
			TokenAccountAddress: tokenAccount.TokenAccountAddress,
			TokenAddress:        tokenAccount.TokenAddress,        // token_address
			TokenDecimal:        int64(tokenAccount.TokenDecimal), // token_decimal
			Balance:             tokenAccount.PostValue,           // token balance
			Slot:                int64(s.slot),                    // 开启统计高度
		}
		tokenAccounts = append(tokenAccounts, solTokenAccount)
	}

	// remove dup
	slice.Reverse(tokenAccounts)
	tokenAccounts = slice.UniqueByComparator[*solmodel.SolTokenAccount](tokenAccounts, func(item *solmodel.SolTokenAccount, other *solmodel.SolTokenAccount) bool {
		if item.OwnerAddress == other.OwnerAddress && item.TokenAccountAddress == other.TokenAccountAddress {
			s.Errorf("SaveTokenAccounts:UniqueByComparator dup token address: %v, account1: %v, account2: %v", item.TokenAddress, item.Balance, other.Balance)
			return true
		}
		return false
	})
	slice.Reverse(tokenAccounts)

	m := make(map[string]time.Time)
	countMap := make(map[string]int)
	slice.ForEach[*solmodel.SolTokenAccount](tokenAccounts, func(_ int, sta *solmodel.SolTokenAccount) {
		if _, ok := m[fmt.Sprintf("%v_%v", sta.ChainId, sta.TokenAddress)]; ok {
			countMap[fmt.Sprintf("%v_%v", sta.ChainId, sta.TokenAddress)] += 1
			return
		}
		address, err := s.sc.TokenModel.FindOneByChainIdAddress(s.ctx, sta.ChainId, sta.TokenAddress)
		if err != nil || address == nil {
			// s.Errorf("SaveTokenAccounts:FindOneByChainIdAddress err:%v， token address: %v", err, sta.TokenAddress)
			return
		}
		m[fmt.Sprintf("%v_%v", sta.ChainId, sta.TokenAddress)] = address.CreatedAt
		countMap[fmt.Sprintf("%v_%v", sta.ChainId, sta.TokenAddress)] = 1
	})

	tokenAccounts = slice.Filter[*solmodel.SolTokenAccount](tokenAccounts, func(_ int, sta *solmodel.SolTokenAccount) bool {
		if value, ok := m[fmt.Sprintf("%v_%v", sta.ChainId, sta.TokenAddress)]; ok {
			sta.CreatedAt = value
			return true
		}
		return false
	})

	err := s.sc.SolTokenAccountModel.BatchInsertTokenAccounts(ctx, tokenAccounts)
	if err != nil {
		s.Error("tokenAccountModel.BatchSave err:", err)
	}
}
