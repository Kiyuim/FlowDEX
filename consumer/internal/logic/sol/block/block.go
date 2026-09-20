package block

import (
	"context"
	"dex/consumer/internal/svc"
	"dex/model/solmodel"
	"dex/pkg/constants"
	"dex/pkg/raydium/clmm"
	"dex/pkg/raydium/cpmm"
	"dex/pkg/sol"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dex/consumer/internal/config"

	"dex/pkg/types"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/blocto/solana-go-sdk/common"
	"github.com/blocto/solana-go-sdk/program/token"
	"github.com/blocto/solana-go-sdk/rpc"
	solTypes "github.com/blocto/solana-go-sdk/types"
	"github.com/decred/base58"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/gorilla/websocket"
	"github.com/panjf2000/ants/v2"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/threading"
)

type BlockService struct {
	Name string
	sc   *svc.ServiceContext
	c    *client.Client
	logx.Logger
	workerPool *ants.Pool
	slotChan   chan uint64
	// holders    *datastructure.CopyOnWriteList[*tokenpkg.ProgramAccount]
	solPrice float64
	slot     uint64
	Conn     *websocket.Conn
	ctx      context.Context
	cancel   func(err error)
	name     string
}

func (s *BlockService) Stop() {
	s.cancel(constants.ErrServiceStop)
	if s.Conn != nil {
		err := s.Conn.WriteMessage(websocket.TextMessage, []byte("{\"id\":1,\"jsonrpc\":\"2.0\",\"method\": \"blockUnsubscribe\", \"params\": [0]}\n"))
		if err != nil {
			s.Error("programUnsubscribe", err)
		}
		_ = s.Conn.Close()
	}
}

func (s *BlockService) Start() {
	s.GetBlockFromHttp()
}

func NewBlockService(sc *svc.ServiceContext, name string, slotChan chan uint64, index int) *BlockService {
	ctx, cancel := context.WithCancelCause(context.Background())
	pool, _ := ants.NewPool(5)
	solService := &BlockService{
		c: client.New(rpc.WithEndpoint(config.FindChainRpcByChainId(constants.SolChainIdInt)), rpc.WithHTTPClient(&http.Client{
			Timeout: 5 * time.Second,
		})),
		// holders:    datastructure.NewCopyOnWriteList[*tokenpkg.ProgramAccount]([]*tokenpkg.ProgramAccount{}),
		sc:         sc,
		Logger:     logx.WithContext(context.Background()).WithFields(logx.Field("service", fmt.Sprintf("%s-%v", name, index))),
		slotChan:   slotChan,
		workerPool: pool,
		ctx:        ctx,
		cancel:     cancel,
		name:       name,
	}
	return solService
}

func (s *BlockService) GetBlockFromHttp() {
	ctx := s.ctx
	for {
		select {
		case <-s.ctx.Done():
			return
		case slot, ok := <-s.slotChan:
			if !ok {
				return
			}
			//打印当前最新slot
			// fmt.Println("current slot is:", slot)
			threading.RunSafe(func() {
				s.ProcessBlock(ctx, int64(slot))
			})
		}
	}
}

func (s *BlockService) ProcessBlock(ctx context.Context, slot int64) {
	beginTime := time.Now()
	s.Logger = s.Logger.WithFields(logx.Field("slot", slot))

	fmt.Println("slot is :", slot)

	s.slot = uint64(slot)

	s.Infof("processBlock:%v start sol consumer will process block slot %v, queue size: %v", slot, slot, len(s.slotChan))

	if slot == 0 {
		s.Errorf("processBlock:%v slot is 0 %v, queue size: %v", slot, slot, len(s.slotChan))
		return
	}

	block, err := s.sc.BlockModel.FindOneBySlot(ctx, slot)
	switch {
	case err != nil && strings.Contains(err.Error(), "record not found"):
		block = &solmodel.Block{
			Slot: slot,
		}
	case err == nil:
		// if block.Status == constants.BlockProcessed || block.Status == constants.BlockSkipped {
		// 	s.Infof("processBlock:%v skip decode, block: %#v", slot, block)
		// 	return
		// }
	default:
		s.Errorf("processBlock:%v findOneBySlot: %v, error: %v", slot, slot, err)
		return
	}

	fmt.Println("slot is:", slot)

	blockInfo, err := GetSolBlockInfo(s.sc.GetSolClient(), ctx, uint64(slot))

	if err != nil || blockInfo == nil {
		if err != nil && strings.Contains(err.Error(), "was skipped") {
			block.Status = constants.BlockSkipped
			s.Infof("processBlock:%v getSolBlockInfo was skipped, err: %v", slot, err)
			_ = s.sc.BlockModel.Insert(ctx, block)
			return
		}
		// 异常区块记录，后续做兜底策略，把丢的区块补回来
		_ = s.sc.BlockModel.Insert(ctx, block)
		s.Errorf("processBlock:%v getSolBlockInfo error: %v", slot, err)
		return
	}

	fmt.Println("blockInfo is:", blockInfo)

	// Set the block time from the retrieved block info
	if blockInfo.BlockTime != nil {
		block.BlockTime = *blockInfo.BlockTime
		blockTime := blockInfo.BlockTime.Format("2006-01-02 15:04:05")
		s.Infof("processBlock:%v getBlockInfo blockTime: %v,cur: %v, dur: %v, queue size: %v", slot, blockTime, time.Now().Format("15:04:05"), time.Since(beginTime), len(s.slotChan))
	} else {
		s.Infof("processBlock:%v getBlockInfo blockTime is nil,cur: %v, dur: %v, queue size: %v", slot, time.Now().Format("15:04:05"), time.Since(beginTime), len(s.slotChan))
	}

	if blockInfo.BlockHeight != nil {
		block.BlockHeight = *blockInfo.BlockHeight
	}
	block.Status = constants.BlockProcessed

	//获取sol 价格
	var tokenAccountMap = make(map[string]*TokenAccount)
	solPrice := s.GetBlockSolPrice(ctx, blockInfo, tokenAccountMap)
	if solPrice == 0 {
		solPrice = s.solPrice
	}

	fmt.Println("sol price is:", solPrice)

	block.SolPrice = solPrice

	// 获取交易并逐个解析
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
			s.Errorf("processBlock:%v decodeTx err:%v, tx:%v", slot, err, decodeTx.TxHash)
			return
		}

		trade = slice.Filter(trade, func(_ int, item *types.TradeWithPair) bool {
			if item == nil {
				fmt.Println("222222222222******222222222222222")
				return false
			}
			s.FillTradeWithPairInfo(item, slot)
			return true
		})

		trades = append(trades, trade...)
		fmt.Println("1111the length of trades is:", len(trades))
	})

	tradeMap := make(map[string][]*types.TradeWithPair)

	raydiumV4Count := 0
	raydiumCPMMCount := 0
	raydiumCLMMCount := 0
	pumpSwapCount := 0
	pumpFunCount := 0

	for _, trade := range trades {
		if len(trade.PairAddr) > 0 {
			tradeMap[trade.PairAddr] = append(tradeMap[trade.PairAddr], trade)
		}
	}

	for _, value := range tradeMap {
		if value[0].SwapName == constants.RaydiumV4 {
			raydiumV4Count++
			continue
		}
		if value[0].SwapName == constants.PumpFun {
			pumpFunCount++
			continue
		}
		if value[0].SwapName == constants.PumpSwap {
			pumpSwapCount++
			continue
		}
		if value[0].SwapName == constants.RaydiumCPMM {
			raydiumCPMMCount++
			continue
		}
		if value[0].SwapName == constants.RaydiumConcentratedLiquidity {
			raydiumCLMMCount++
			continue
		}
	}

	s.Logger.Infof("processBlock:%v trade size: %v,raydiumCPMMCount: %v, CLMMCount: %v, SWAPCount: %v", slot, len(trades), raydiumCPMMCount, raydiumCLMMCount, pumpSwapCount)

	{
		tokenMints := slice.Filter[*types.TradeWithPair](trades, func(_ int, item *types.TradeWithPair) bool {
			if item != nil && item.Type == types.TradeTokenMint {
				return true
			}
			return false
		})

		s.UpdateTokenMints(ctx, tokenMints)
		s.Infof("processBlock:%v UpdateTokenMints size: %v, dur: %v, tokenMints: %v", slot, len(tokenMints), time.Since(beginTime), len(tokenMints))
	}

	// {
	// 	tokenBurns := slice.Filter[*types.TradeWithPair](trades, func(_ int, item *types.TradeWithPair) bool {
	// 		if item != nil && item.Type == types.TradeTokenBurn {
	// 			return true
	// 		}
	// 		return false
	// 	})

	// 	s.UpdateTokenBurns(ctx, tokenBurns)
	// 	s.Infof("processBlock:%v UpdateTokenBurns size: %v, dur: %v, tokenMints: %v", slot, len(tokenBurns), time.Since(beginTime), len(tokenBurns))
	// }

	// 推送 sol 和 代币价格给trade服务，用于限价单交易匹配
	s.SendTokenPrice2TradeRPC(ctx, solPrice, tradeMap)

	//并发处理： 保存交易信息，保存token账户信息
	group := threading.NewRoutineGroup()
	group.RunSafe(func() {
		s.SaveTrades(ctx, constants.SolChainIdInt, tradeMap)
		s.Infof("processBlock:%v saveTrades tx_size: %v, dur: %v, trade_size: %v", slot, len(blockInfo.Transactions), time.Since(beginTime), len(trades))

		fmt.Println("it's going to send meessages to kafka")
		//Kafka 推送交易消息
		s.SendTx(ctx, slot, trades)
		// s.Infof("processBlock:%v sendTx tx_size: %v, dur: %v, trade_size: %v", slot, len(blockInfo.Transactions), time.Since(beginTime), len(trades))

		s.SaveTokenAccounts(ctx, trades, tokenAccountMap)
		s.Infof("processBlock:%v saveTokenAccounts tx_size: %v, dur: %v, trade_size: %v", slot, len(blockInfo.Transactions), time.Since(beginTime), len(trades))
	})

	// clmm
	fmt.Println("CLMM *************")
	fmt.Println("the length of trades is:", len(trades))
	group.RunSafe(func() {
		slice.ForEach(trades, func(_ int, trade *types.TradeWithPair) {
			fmt.Println("trade swapName is:", trade.SwapName)

			if trade.SwapName == constants.RaydiumConcentratedLiquidity || trade.SwapName == "RaydiumClmm" {
				s.Infof("CLMM Processing: Found CLMM trade with type: %s, txHash: %s, pair: %s",
					trade.Type, trade.TxHash, trade.PairAddr)

				// Save all CLMM trades, not just buy/sell
				if err = s.SaveRaydiumCLMMPoolInfo(ctx, trade); err != nil {
					s.Errorf("processBlock:%v saveRaydiumCLMMPoolInfo err: %v, trade.Type: %s", slot, err, trade.Type)
				} else {
					s.Infof("CLMM Success: Saved pool info for txHash: %s, trade.Type: %s", trade.TxHash, trade.Type)
				}
			}
		})
	})

	// cpmm
	// group.RunSafe(func() {
	// 	slice.ForEach(trades, func(_ int, trade *types.TradeWithPair) {
	// 		if trade.SwapName == constants.RaydiumCPMM || trade.SwapName == "RaydiumCpmm" {
	// 			if trade.Type == types.TradeTypeBuy || trade.Type == types.TradeTypeSell {
	// 				if err = s.SaveRaydiumCPMMPoolInfo(ctx, trade); err != nil {
	// 					s.Errorf("processBlock:%v SaveRaydiumCPMMPoolInfo err: %v", slot, err)
	// 				}
	// 			}

	// 		}
	// 	})
	// })

	// // pump swap
	group.RunSafe(func() {
		slice.ForEach(trades, func(_ int, trade *types.TradeWithPair) {
			if trade.SwapName == constants.PumpSwap || trade.SwapName == "PumpSwap" {
				if trade.Type == types.TradeTypeBuy || trade.Type == types.TradeTypeSell {
					if err = s.SavePumpSwapPoolInfo(ctx, trade); err != nil {
						s.Errorf("processBlock:%v SavePumpSwapPoolInfo err: %v", slot, err)
					}
				}

			}
		})
	})

	// // raydium v4
	// group.RunSafe(func() {
	// 	poolMap := make(map[string]*solmodel.RaydiumPool)
	// 	for _, trade := range trades {
	// 		if trade.RaydiumPool != nil {
	// 			// raydium init without serum asks
	// 			if len(trade.RaydiumPool.SerumAsks) == 0 {
	// 				continue
	// 			}
	// 			// exists
	// 			if _, ok := poolMap[trade.RaydiumPool.AmmId]; ok {
	// 				continue
	// 			}
	// 			poolMap[trade.RaydiumPool.AmmId] = trade.RaydiumPool
	// 		}
	// 	}
	// 	s.BatchSaveRaydiumPool(ctx, poolMap)
	// 	s.Infof("processBlock:%v saveRaydiumPool tx_size: %v, dur: %v, raydiumV4 count: %v, pool_map_size: %v", slot, len(blockInfo.Transactions), time.Since(beginTime), raydiumV4Count, len(poolMap))
	// })

	group.Wait()

	err = s.sc.BlockModel.Insert(ctx, block)
	if err != nil {
		s.Errorf("processBlock:%v blockModel update err:", slot, err, block)
		return
	}
}

func GetInnerInstructionMap(tx *client.BlockTransaction) map[int]*client.InnerInstruction {
	// innerInstruction map
	var innerInstructionMap = make(map[int]*client.InnerInstruction)
	for i := range tx.Meta.InnerInstructions {
		innerInstructionMap[int(tx.Meta.InnerInstructions[i].Index)] = &tx.Meta.InnerInstructions[i]
	}
	return innerInstructionMap
}

func FillTokenAccountMap(tx *client.BlockTransaction, tokenAccountMapIn map[string]*TokenAccount) (tokenAccountMap map[string]*TokenAccount, hasTokenChange bool) {
	if tokenAccountMapIn == nil {
		tokenAccountMapIn = make(map[string]*TokenAccount)
	}
	tokenAccountMap = tokenAccountMapIn
	for _, pre := range tx.Meta.PreTokenBalances {
		var tokenAccount = tx.AccountKeys[pre.AccountIndex].String()
		preValue, _ := strconv.ParseInt(pre.UITokenAmount.Amount, 10, 64)
		tokenAccountMap[tokenAccount] = &TokenAccount{
			Owner:               pre.Owner,                  // owner address
			TokenAccountAddress: tokenAccount,               // token account address
			TokenAddress:        pre.Mint,                   // token address
			TokenDecimal:        pre.UITokenAmount.Decimals, // token decimal
			PreValue:            preValue,
			Closed:              true,
			PreValueUIString:    pre.UITokenAmount.UIAmountString,
		}
	}
	for _, post := range tx.Meta.PostTokenBalances {
		var tokenAccount = tx.AccountKeys[post.AccountIndex].String()
		postValue, _ := strconv.ParseInt(post.UITokenAmount.Amount, 10, 64)
		if tokenAccountMap[tokenAccount] != nil {
			tokenAccountMap[tokenAccount].Closed = false
			tokenAccountMap[tokenAccount].PostValue = postValue
			if tokenAccountMap[tokenAccount].PostValue != tokenAccountMap[tokenAccount].PreValue {
				hasTokenChange = true
			}
		} else {
			hasTokenChange = true
			tokenAccountMap[tokenAccount] = &TokenAccount{
				Owner:               post.Owner,                  // owner address
				TokenAccountAddress: tokenAccount,                // token account address
				TokenAddress:        post.Mint,                   // token address
				TokenDecimal:        post.UITokenAmount.Decimals, // token decimal
				PostValue:           postValue,                   // token balance
				Init:                true,
				PostValueUIString:   post.UITokenAmount.UIAmountString,
			}
		}
	}
	for i := range tx.Transaction.Message.Instructions {
		instruction := &tx.Transaction.Message.Instructions[i]
		program := tx.AccountKeys[instruction.ProgramIDIndex].String()
		if program == ProgramStrToken {
			DecodeInitAccountInstruction(tx, tokenAccountMap, instruction)
		}
	}
	for _, instructions := range tx.Meta.InnerInstructions {
		for i := range instructions.Instructions {
			instruction := instructions.Instructions[i]
			program := tx.AccountKeys[instruction.ProgramIDIndex].String()
			if program == ProgramStrToken {
				DecodeInitAccountInstruction(tx, tokenAccountMap, &instruction)
			}
		}
	}
	tokenDecimalMap := make(map[string]uint8)
	for _, v := range tokenAccountMap {
		if v.TokenDecimal != 0 {
			tokenDecimalMap[v.TokenAddress] = v.TokenDecimal
		}
	}
	for _, v := range tokenAccountMap {
		if v.TokenDecimal == 0 {
			v.TokenDecimal = tokenDecimalMap[v.TokenAddress]
		}
	}
	return
}

func GetInnerInstructionByInner(instructions []solTypes.CompiledInstruction, startIndex, innerLen int) *client.InnerInstruction {
	if startIndex+innerLen+1 > len(instructions) {
		return nil
	}
	innerInstruction := &client.InnerInstruction{
		Index: uint64(instructions[startIndex].ProgramIDIndex),
	}
	for i := 0; i < innerLen; i++ {
		innerInstruction.Instructions = append(innerInstruction.Instructions, instructions[startIndex+i+1])
	}
	return innerInstruction
}

func DecodeTokenTransfer(accountKeys []common.PublicKey, instruction *solTypes.CompiledInstruction) (transfer *token.TransferParam, err error) {
	transfer = &token.TransferParam{}
	if accountKeys[instruction.ProgramIDIndex].String() == common.Token2022ProgramID.String() {
		if len(instruction.Accounts) < 3 {
			err = errors.New("not enough accounts")
			return
		}
		if len(instruction.Data) < 1 {
			err = errors.New("data len to0 small")
			return
		}
		if instruction.Data[0] == byte(token.InstructionTransfer) {
			if len(instruction.Data) != 9 {
				err = errors.New("data len not equal 9")
				return
			}
			if len(instruction.Accounts) < 3 {
				err = errors.New("account len too small")
				return
			}
			transfer.From = accountKeys[instruction.Accounts[0]]
			transfer.To = accountKeys[instruction.Accounts[1]]
			transfer.Auth = accountKeys[instruction.Accounts[2]]
			transfer.Amount = binary.LittleEndian.Uint64(instruction.Data[1:])
		} else if instruction.Data[0] == byte(token.InstructionTransferChecked) {
			if len(instruction.Data) < 10 {
				err = errors.New("data len not equal 10")
				return
			}
			if len(instruction.Accounts) < 4 {
				err = errors.New("account len too small")
				return
			}
			transfer.From = accountKeys[instruction.Accounts[0]]
			// mint := accountKeys[instruction.Accounts[1]]
			transfer.To = accountKeys[instruction.Accounts[2]]
			transfer.Auth = accountKeys[instruction.Accounts[3]]
			transfer.Amount = binary.LittleEndian.Uint64(instruction.Data[1:10])
			// decimal := instruction.Data[10]
		} else {
			err = errors.New("not transfer Instruction")
			return
		}
		return transfer, nil
	}

	if accountKeys[instruction.ProgramIDIndex].String() != ProgramStrToken {
		err = errors.New("not token program")
		return
	}
	if len(instruction.Accounts) < 3 {
		err = errors.New("not enough accounts")
		return
	}
	if len(instruction.Data) < 1 {
		err = errors.New("data len to0 small")
		return
	}
	if instruction.Data[0] == byte(token.InstructionTransfer) {
		if len(instruction.Data) != 9 {
			err = errors.New("data len not equal 9")
			return
		}
		if len(instruction.Accounts) < 3 {
			err = errors.New("account len too small")
			return
		}
		transfer.From = accountKeys[instruction.Accounts[0]]
		transfer.To = accountKeys[instruction.Accounts[1]]
		transfer.Auth = accountKeys[instruction.Accounts[2]]
		transfer.Amount = binary.LittleEndian.Uint64(instruction.Data[1:])
	} else if instruction.Data[0] == byte(token.InstructionTransferChecked) {
		if len(instruction.Data) != 10 {
			err = errors.New("data len not equal 10")
			return
		}
		if len(instruction.Accounts) < 4 {
			err = errors.New("account len too small")
			return
		}
		transfer.From = accountKeys[instruction.Accounts[0]]
		// mint := accountKeys[instruction.Accounts[1]]
		transfer.To = accountKeys[instruction.Accounts[2]]
		transfer.Auth = accountKeys[instruction.Accounts[3]]
		transfer.Amount = binary.LittleEndian.Uint64(instruction.Data[1:10])
		// decimal := instruction.Data[10]
	} else {
		err = errors.New("not transfer Instruction")
		return
	}

	return
}

func DecodeInitAccountInstruction(tx *client.BlockTransaction, tokenAccountMap map[string]*TokenAccount, instruction *solTypes.CompiledInstruction) {
	if len(instruction.Data) == 0 {
		return
	}
	var mint, tokenAccount, owner string
	switch token.Instruction(instruction.Data[0]) {
	// init account
	case token.InstructionInitializeAccount:
		if len(instruction.Accounts) < 3 {
			return
		}
		tokenAccount = tx.AccountKeys[instruction.Accounts[0]].String()
		mint = tx.AccountKeys[instruction.Accounts[1]].String()
		owner = tx.AccountKeys[instruction.Accounts[2]].String()
	case token.InstructionInitializeAccount2:
		if len(instruction.Accounts) < 2 || len(instruction.Data) < 33 {
			return
		}
		tokenAccount = tx.AccountKeys[instruction.Accounts[0]].String()
		mint = tx.AccountKeys[instruction.Accounts[1]].String()
		owner = common.PublicKeyFromBytes(instruction.Data[1:]).String()
	case token.InstructionInitializeAccount3:
		if len(instruction.Accounts) < 2 || len(instruction.Data) < 33 {
			return
		}
		tokenAccount = tx.AccountKeys[instruction.Accounts[0]].String()
		mint = tx.AccountKeys[instruction.Accounts[1]].String()
		owner = common.PublicKeyFromBytes(instruction.Data[1:]).String()
	default:
		return
	}
	if tokenAccountMap[tokenAccount] != nil && tokenAccountMap[tokenAccount].TokenAddress == mint {
		return
	} else {
		tokenAccountMap[tokenAccount] = &TokenAccount{
			Init:                true,
			Owner:               owner,
			TokenAddress:        mint,
			TokenAccountAddress: tokenAccount,
			TokenDecimal:        0,
			PreValue:            0,
			PostValue:           0,
		}
	}
}

func DecodeTx(ctx context.Context, sc *svc.ServiceContext, dtx *DecodedTx) (trades []*types.TradeWithPair, err error) {
	if dtx.Tx == nil || dtx.BlockDb == nil {
		return
	}
	tx := dtx.Tx
	dtx.TxHash = base58.Encode(tx.Transaction.Signatures[0])

	// check tx err
	if tx.Meta.Err != nil {
		// s.Errorf("tx %s exec err: %v", txHash, tx.Meta.Err)
		return
	}

	fmt.Println("tx.Meta.LogMessages", tx.Meta.LogMessages)
	dtx.InnerInstructionMap = GetInnerInstructionMap(tx)

	if len(tx.Meta.LogMessages) == 0 {
		return nil, fmt.Errorf("decode tx maybe vote tx, tx hash: %v", dtx.TxHash)
	}

	// instructions
	for i := range tx.Transaction.Message.Instructions {
		instruction := &tx.Transaction.Message.Instructions[i]
		var trade *types.TradeWithPair
		trade, err = DecodeInstruction(ctx, sc, dtx, instruction, i)
		if err == nil && trade != nil {
			trades = append(trades, trade)
			continue
		}
		// Only log errors for known error types, ignore unknown instruction types
		if err != nil && !errors.Is(err, ErrTokenAmountIsZero) && !errors.Is(err, ErrNotSupportWarp) && !errors.Is(err, ErrNotSupportInstruction) && !errors.Is(err, ErrUnknownProgram) {
			err = fmt.Errorf("decodeInstruction err:%v", err)
			logx.Error(err)
		}
	}

	if len(trades) > 0 {
		fmt.Println("444444444444******444444444444444")
		fmt.Println("the length of trades is:", len(trades))
		fmt.Println("err is:", err)
		err = nil
	}

	// inner instructions
	// for i := range tx.Meta.InnerInstructions {
	// 	for j := range tx.Meta.InnerInstructions[i].Instructions {
	// 		instruction := &tx.Meta.InnerInstructions[i].Instructions[j]
	// 		var trade *types.TradeWithPair
	// 		trade, err = DecodeInnerInstruction(ctx, sc, dtx, instruction, int(tx.Meta.InnerInstructions[i].Index), j)
	// 		if err == nil && trade != nil {
	// 			trades = append(trades, trade)
	// 			continue
	// 		}
	// 		// Only log errors for known error types, ignore unknown instruction types
	// 		if err != nil && !errors.Is(err, ErrTokenAmountIsZero) && !errors.Is(err, ErrNotSupportWarp) && !errors.Is(err, ErrNotSupportInstruction) && !errors.Is(err, ErrUnknownProgram) {
	// 			err = fmt.Errorf("decodeInnerInstruction err:%v", err)
	// 			logx.Error(err)
	// 		}
	// 	}
	// }

	return
}

func DecodeInstruction(
	ctx context.Context,
	sc *svc.ServiceContext,
	dtx *DecodedTx,
	instruction *solTypes.CompiledInstruction,
	index int,
) (trade *types.TradeWithPair, err error) {
	if len(dtx.Tx.AccountKeys) == 0 {
		return nil, fmt.Errorf("account keys are empty")
	}

	if int(instruction.ProgramIDIndex) >= len(dtx.Tx.AccountKeys) {
		return nil, fmt.Errorf("program ID index %d out of bounds for account keys length %d", instruction.ProgramIDIndex, len(dtx.Tx.AccountKeys))
	}

	tx := dtx.Tx
	// var innerInstructions *client.InnerInstruction
	program := tx.AccountKeys[instruction.ProgramIDIndex].String()

	if program == clmm.ProgramClMMDevNet.String() || program == "A1izdbCxDvLjZ2WZFkPdSLNBrrYrhBqxmmzCkm82G4ys" {
		fmt.Println("22222DecodeInstruction: Processing program %s for tx %s", program, dtx.TxHash)
	}

	if program == ProgramStrPumpAmm {
		decoder := &PumpAmmDecoder{
			ctx:                 ctx,
			svcCtx:              sc,
			dtx:                 dtx,
			compiledInstruction: instruction,
		}
		trade, err = decoder.DecodePumpAmmInstruction()
		if trade != nil {
			fmt.Println("DecodePumpAmmInstruction trade pair name is:", trade.PairInfo.Name)
		}
		return trade, err
	} else if program == ProgramStrPumpFun {
		logx.Infof("Find pump fun tx: %v", dtx.TxHash)
		trade, err = DecodePumpInstruction(ctx, sc, dtx, instruction, index)
		fmt.Println("333333333333******333333333333333")
		fmt.Println("trade is:", trade)
		fmt.Println("err is:", err)
		return trade, err
	} else if program == ProgramStrRaydiumV4 {
		// innerInstructions = dtx.InnerInstructionMap[index]
		// if innerInstructions == nil {
		// 	err = fmt.Errorf("RaydiumV4 swap without inner instruction tx hash:%v", dtx.TxHash)
		// 	return
		// }
		// trade, err = DecodeRaydiumInstruction(ctx, sc, dtx, instruction, innerInstructions)
		// return trade, err
	} else if program == clmm.ProgramClMMDevNet.String() || program == "A1izdbCxDvLjZ2WZFkPdSLNBrrYrhBqxmmzCkm82G4ys" { // devnet clmm
		innerInstructions := dtx.InnerInstructionMap[index]
		decoder := &ConcentratedLiquidityDecoder{
			ctx:                 ctx,
			svcCtx:              sc,
			dtx:                 dtx,
			compiledInstruction: instruction,
			innerInstruction:    innerInstructions,
		}
		trade, err = decoder.DecodeRaydiumConcentratedLiquidityInstruction()
		if err != nil {
			logx.Errorf("error find clmm tx: %v, err : %v", dtx.TxHash, err)
			return nil, err
		}
		return trade, err
	} else if program == ProgramStrPumpMeteora {
		// second token source: our own pump-meteora bonding-curve program
		trade, err = DecodePumpMeteoraInstruction(ctx, sc, dtx, instruction, index)
		return trade, err
	} else if program == ProgramStrPumpMeteoraOpt {
		// third token source: the optimized/hardened pump-meteora build
		trade, err = DecodePumpMeteoraOptInstruction(ctx, sc, dtx, instruction, index)
		return trade, err
	} else if program == cpmm.ProgramRaydiumCPMMProgram.String() {
		// innerInstructions = dtx.InnerInstructionMap[index]
		// decoder := &CPMMDecoder{
		// 	ctx:                 ctx,
		// 	svcCtx:              sc,
		// 	dtx:                 dtx,
		// 	compiledInstruction: instruction,
		// 	innerInstruction:    innerInstructions,
		// }
		// trade, err = decoder.DecodeCPMMInstruction()
		// if err != nil {
		// 	logx.Errorf("error find cpmm tx: %v, err : %v", dtx.TxHash, err)
		// 	return nil, err
		// }
		// logx.Infof("find cpmm tx: %v, pairInfo: %#v", dtx.TxHash, trade.PairInfo)
		// return trade, err
	} else if program == common.TokenProgramID.String() {
		// trade, err = DecodeTokenProgramInstruction(ctx, sc, dtx, instruction, index)

		// if trade != nil {
		// 	fmt.Println("find token program tx: %v", trade.TxHash)
		// }
		// return trade, err
	} else if program == common.Token2022ProgramID.String() {
		// innerInstructions = dtx.InnerInstructionMap[index]
		// decoder := &Token2022Decoder{
		// 	ctx:                 ctx,
		// 	svcCtx:              sc,
		// 	dtx:                 dtx,
		// 	compiledInstruction: instruction,
		// 	innerInstruction:    innerInstructions,
		// }
		// trade, err = decoder.DecodeToken2022DecoderInstruction()
		// if err != nil {
		// 	logx.Errorf("error find token2022 tx: %v, err : %v", dtx.TxHash, err)
		// 	return nil, err
		// }
		// logx.Infof("find token2022 tx: %v, pairInfo: %#v", dtx.TxHash, trade.PairInfo)
		// return trade, err
	}

	// Return error for unknown programs instead of nil, nil
	return nil, ErrUnknownProgram
}

func DecodeInnerInstruction(
	ctx context.Context,
	sc *svc.ServiceContext,
	dtx *DecodedTx,
	instruction *solTypes.CompiledInstruction,
	i, j int,
) (trade *types.TradeWithPair, err error) {
	tx := dtx.Tx
	var innerInstructions *client.InnerInstruction
	program := tx.AccountKeys[instruction.ProgramIDIndex].String()

	if program == ProgramStrPumpAmm {
		decoder := &PumpAmmDecoder{
			ctx:                 ctx,
			svcCtx:              sc,
			dtx:                 dtx,
			compiledInstruction: instruction,
		}
		trade, _ = decoder.DecodePumpAmmInstruction()
		return trade, nil
	} else if program == ProgramStrRaydiumV4 {
		innerInstructions, err = GetRaydiumSwapInnerInstruction(dtx, i, j)
		if err != nil {
			err = fmt.Errorf("GetRaydiumSwapInnerInstruction err:%w", err)
			return
		}
		trade, err = DecodeRaydiumInstruction(ctx, sc, dtx, instruction, innerInstructions)
		if err != nil {
			err = fmt.Errorf("DecodeRaydiumInstruction err:%w", err)
			return
		}
	} else if program == ProgramStrPumpFun {
		trade, err = DecodePumpInstruction(ctx, sc, dtx, instruction, i)
		return trade, err
	} else if program == clmm.ProgramRaydiumConcentratedLiquidity.String() {
		// https://solscan.io/tx/2drrExUH4L8QCsoJPW8FFkKQVeLpuNiY4wqrfKDp8NwnWkfk5GkygSX2oKXp2NPQr7dFcVdhpUNLamcJEgHVcn1x
		innerInstructions, err = GetRaydiumSwapInnerInstruction(dtx, i, j)
		decoder := &ConcentratedLiquidityDecoder{
			ctx:                 ctx,
			svcCtx:              sc,
			dtx:                 dtx,
			compiledInstruction: instruction,
			innerInstruction:    innerInstructions,
		}
		trade, err = decoder.DecodeRaydiumConcentratedLiquidityInstruction()
		if err != nil {
			logx.Errorf("error find inner clmm tx: %v, err : %v", dtx.TxHash, err)
			return nil, err
		}
		logx.Infof("find inner clmm tx: %v, pairInfo: %#v", dtx.TxHash, trade.PairInfo)
		return trade, nil
	} else if program == cpmm.ProgramRaydiumCPMMProgram.String() {
		innerInstructions, err = GetRaydiumSwapInnerInstruction(dtx, i, j)
		decoder := &CPMMDecoder{
			ctx:                 ctx,
			svcCtx:              sc,
			dtx:                 dtx,
			compiledInstruction: instruction,
			innerInstruction:    innerInstructions,
		}
		trade, err = decoder.DecodeCPMMInstruction()
		if err != nil {
			logx.Errorf("error find inner cpmm tx: %v, err : %v", dtx.TxHash, err)
			return nil, err
		}
		logx.Infof("find inner cpmm tx: %v, pairInfo: %#v", dtx.TxHash, trade.PairInfo)
		return trade, err
	} else if program == ProgramStrPumpMeteora {
		trade, err = DecodePumpMeteoraInstruction(ctx, sc, dtx, instruction, i)
		return trade, err
	} else if program == ProgramStrPumpMeteoraOpt {
		trade, err = DecodePumpMeteoraOptInstruction(ctx, sc, dtx, instruction, i)
		return trade, err
	}

	// Return error for unknown programs instead of nil, nil
	return nil, ErrUnknownProgram
}

func GetTokenDecimal(ctx context.Context, sc *svc.ServiceContext, address string) (tokenDecimal uint8, err error) {
	if address == TokenStrWrapSol {
		tokenDecimal = 9
		return
	}
	var errMysql, errRpc error
	tokenDecimal, errMysql = GetTokenDecimalByMysql(ctx, sc, address)
	if errMysql != nil {
		tokenDecimal, errRpc = GetTokenAccountDecimalByRpc(ctx, sc, address)
		if errRpc != nil {
			err = fmt.Errorf("GetTokenAccountDecimal err:mysql(%w), rpc(%w)", errMysql, errRpc)
		}
	}
	return
}

func GetTokenDecimalByMysql(ctx context.Context, sc *svc.ServiceContext, address string) (tokenDecimal uint8, err error) {
	tokenModel := sc.TokenModel
	var Token *solmodel.Token
	Token, err = tokenModel.FindOneByChainIdAddress(ctx, SolChainIdInt, address)
	if err != nil {
		err = fmt.Errorf("FindOneByChainIdAddressNoCache err:%w", err)
		return
	}
	tokenDecimal = uint8(Token.Decimals)
	if tokenDecimal == 0 {
		err = errors.New("tokenDecimal is zero")
	}
	return
}

func GetTokenAccountDecimalByRpc(ctx context.Context, sc *svc.ServiceContext, address string) (tokenDecimal uint8, err error) {
	// c := sc.SolClient
	c := sc.GetSolClient()

	tokeInfo, err := sol.GetTokenInfo(c, ctx, address)
	if err != nil {
		err = fmt.Errorf("GetTokenAccountBalanceAndContext err:%w", err)
		return
	}
	tokenDecimal = tokeInfo.Decimals
	return
}

func GetRaydiumSwapInnerInstruction(dtx *DecodedTx, i, j int) (innerInstructions *client.InnerInstruction, err error) {
	instructions := dtx.InnerInstructionMap[i]
	if instructions == nil {
		err = errors.New("innerInstructions not found")
		return
	}
	if len(instructions.Instructions)-1 < j+2 {
		err = errors.New("no enough instruction")
		return
	}
	innerInstructions = &client.InnerInstruction{
		Index: uint64(j),
	}
	innerInstructions.Instructions = append(innerInstructions.Instructions, instructions.Instructions[j+1])
	innerInstructions.Instructions = append(innerInstructions.Instructions, instructions.Instructions[j+2])
	return
}
