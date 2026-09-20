package block

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dex/pkg/solprice"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/blocto/solana-go-sdk/common"
	"github.com/blocto/solana-go-sdk/program/token"
	"github.com/blocto/solana-go-sdk/rpc"
	"github.com/decred/base58"
)

var ProgramOrca = common.PublicKeyFromString("whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc")
var ProgramRaydiumConcentratedLiquidity = common.PublicKeyFromString("CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK")
var ProgramMeteoraDLMM = common.PublicKeyFromString("LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo")
var ProgramPhoneNix = common.PublicKeyFromString("PhoeNiXZ8ByJGLkxNfZRnkUfjvmuYqLR89jjFHGqdXY")

var StableCoinSwapDexes = []common.PublicKey{ProgramOrca, ProgramRaydiumConcentratedLiquidity, ProgramMeteoraDLMM, ProgramPhoneNix}

func GetSolBlockInfoDelay(c *client.Client, ctx context.Context, slot uint64) (resp *client.Block, err error) {
	// 减少helius调用，因为失败也算次数
	time.Sleep(time.Second * 1)
	return GetSolBlockInfo(c, ctx, slot)
}

func GetSolBlockInfo(c *client.Client, ctx context.Context, slot uint64) (resp *client.Block, err error) {
	var count int64
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		resp, err = c.GetBlockWithConfig(ctx, slot, client.GetBlockConfig{
			Commitment:         rpc.CommitmentConfirmed,
			TransactionDetails: rpc.GetBlockConfigTransactionDetailsFull,
		})
		switch {
		case err == nil:
			return
		case strings.Contains(err.Error(), "Block not available for slot"):
			count++
			if count > 10 {
				return
			}
			time.Sleep(time.Second)
		case strings.Contains(err.Error(), "limit"):
			count++
			if count > 10 {
				return
			}
			time.Sleep(time.Second)
		default:
			err = fmt.Errorf("GetBlock err:%w", err)
			return
		}
	}
}

func (s *BlockService) GetBlockSolPrice(ctx context.Context, block *client.Block, tokenAccountMap map[string]*TokenAccount) float64 {
	priceList := make([]float64, 0)
	if tokenAccountMap == nil {
		tokenAccountMap = make(map[string]*TokenAccount)
	}

	for i := range block.Transactions {
		tx := &block.Transactions[i]
		hash := base58.Encode(tx.Transaction.Signatures[0]) // todo: delete me
		_ = hash
		accountKeys := tx.AccountKeys
		innerInstructionMap := GetInnerInstructionMap(tx)
		tokenAccountMap, hasChange := FillTokenAccountMap(tx, tokenAccountMap)
		if !hasChange {
			continue
		}
		for _, instruction := range tx.Transaction.Message.Instructions {
			if in(StableCoinSwapDexes, accountKeys[instruction.ProgramIDIndex]) {
				price := GetBlockSolPriceByTransfer(accountKeys, innerInstructionMap[instruction.ProgramIDIndex], tokenAccountMap)
				if price > 0 {
					priceList = append(priceList, price)
				}
			}
		}
		for _, instructions := range tx.Meta.InnerInstructions {
			for i, instruction := range instructions.Instructions {
				if in(StableCoinSwapDexes, accountKeys[instruction.ProgramIDIndex]) {
					innerInstruction := GetInnerInstructionByInner(instructions.Instructions, i, 2)
					price := GetBlockSolPriceByTransfer(accountKeys, innerInstruction, tokenAccountMap)
					if price > 0 {
						priceList = append(priceList, price)
					}
				}
			}
		}
	}

	price := RemoveMinAndMaxAndCalculateAverage(priceList)

	if price > 0 {
		return price
	}
	if s.solPrice > 0 {
		return s.solPrice
	}
	b, err := s.sc.BlockModel.FindOneByNearSlot(s.ctx, int64(block.ParentSlot))
	if err == nil && b != nil && b.SolPrice > 0 {
		return b.SolPrice
	}
	// devnet has no real stablecoin pool to derive a price from, so every
	// source above is always 0 here — and TokenPriceUSD computed from a
	// zero SOL price is always 0 too, which SaveTrades silently filters
	// out before a single real trade is ever saved. Fall back to the same
	// CoinGecko-backed price the market service already uses for display.
	if p := solprice.GetSolUsdPrice(); p > 0 {
		return p
	}
	return 0
}

func in[T comparable](list []T, a T) bool {
	for i := 0; i < len(list); i++ {
		if list[i] == a {
			return true
		}
	}
	return false
}

func GetBlockSolPriceByTransfer(accountKeys []common.PublicKey, innerInstructions *client.InnerInstruction, tokenAccountMap map[string]*TokenAccount) (solPrice float64) {
	if innerInstructions == nil {
		return
	}
	var transferSOL *token.TransferParam
	var transferUSD *token.TransferParam
	var connect bool
	for j := range innerInstructions.Instructions {
		transfer, err := DecodeTokenTransfer(accountKeys, &innerInstructions.Instructions[j])
		if err != nil {
			// err = fmt.Errorf("DecodeTokenTransfer err:%w", err)
			transferSOL = nil
			transferUSD = nil
			connect = false
			continue
		}
		from := tokenAccountMap[transfer.From.String()]
		if from == nil {
			transferSOL = nil
			transferUSD = nil
			connect = false
			continue
		}
		to := tokenAccountMap[transfer.To.String()]
		if to == nil {
			transferSOL = nil
			transferUSD = nil
			connect = false
			continue
		}
		if from.TokenAddress == TokenStrWrapSol {
			transferSOL = transfer
			if connect && transferUSD != nil {
				solPrice = float64(transferUSD.Amount) / float64(transferSOL.Amount) * 1000
				if IsSwapTransfer(transferSOL, transferUSD, tokenAccountMap) {
					break
				} else {
					transferUSD = nil
				}
			}
			connect = true
		} else if from.TokenAddress == TokenStrUSDC || from.TokenAddress == TokenStrUSDT {
			transferUSD = transfer
			if connect && transferSOL != nil {
				solPrice = float64(transferUSD.Amount) / float64(transferSOL.Amount) * 1000
				if IsSwapTransfer(transferSOL, transferUSD, tokenAccountMap) {
					break
				} else {
					transferSOL = nil
				}
			}
			connect = true
		} else {
			transferSOL = nil
			transferUSD = nil
			connect = false
		}
	}
	if transferSOL != nil && transferUSD != nil && connect {
		solPrice = float64(transferUSD.Amount) / float64(transferSOL.Amount) * 1000
	} else {
		solPrice = 0
	}
	return
}

func IsSwapTransfer(a, b *token.TransferParam, tokenAccountMap map[string]*TokenAccount) bool {
	if a == nil || b == nil {
		return false
	}
	aFrom := tokenAccountMap[a.From.String()]
	aTo := tokenAccountMap[a.To.String()]
	bFrom := tokenAccountMap[b.From.String()]
	bTo := tokenAccountMap[b.To.String()]
	if aFrom == nil || aTo == nil || bFrom == nil || bTo == nil {
		return false
	}
	if aFrom.Owner == bTo.Owner {
		return true
	}
	if bFrom.Owner == aTo.Owner {
		return true
	}
	return false
}
