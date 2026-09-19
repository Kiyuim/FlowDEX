package block

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"dex/consumer/internal/svc"
	"dex/pkg/types"

	"github.com/blocto/solana-go-sdk/program/tokenprog"
	solTypes "github.com/blocto/solana-go-sdk/types"

	"golang.org/x/net/context"
)

func DecodeTokenProgramInstruction(ctx context.Context, sc *svc.ServiceContext, dtx *DecodedTx, instruction *solTypes.CompiledInstruction, index int) (trade *types.TradeWithPair, err error) {
	if len(instruction.Data) == 0 {
	}
	switch tokenprog.Instruction(instruction.Data[0]) {
	case tokenprog.InstructionMintTo:
		// https://solscan.io/tx/3joVrvngnR3SmMAC4NmQXW6BvAr9UhMh8o2VncsyAR78V91M8RNbjU6q8TDLkHjYJRLXPparvq3E4m1V6TudTXv1
		return DecodeInstructionMintTo(ctx, sc, dtx, instruction)
	case tokenprog.InstructionBurn:
		// https://solscan.io/tx/5PzVCFdJuFH7pFor1ybETAJ9qKUMZdVxJjPgqGNu4kchxTB47LLNshj2r1SCMoaVyNCbSo7rxzbwqtR7vCmkmXoM
		// https://solscan.io/token/DDGcYJkMMD1iiLRfPQLZePxLJCLDhiioQ83frmdAJd3h?activity_type=ACTIVITY_SPL_BURN
		return DecodeInstructionBurn(ctx, sc, dtx, instruction)
	default:
		return nil, nil
	}
}

func DecodeInstructionMintTo(_ context.Context, _ *svc.ServiceContext, dtx *DecodedTx, instr *solTypes.CompiledInstruction) (trade *types.TradeWithPair, err error) {

	tx := dtx.Tx
	// 提取 Mint 地址
	mintIndex := instr.Accounts[0]
	if mintIndex >= len(tx.AccountKeys) {
		return nil, fmt.Errorf("invalid token program index")
	}
	mint := tx.AccountKeys[mintIndex]

	toIndex := instr.Accounts[1]
	if mintIndex >= len(tx.AccountKeys) {
		return nil, fmt.Errorf("invalid token program index")
	}
	to := tx.AccountKeys[toIndex]

	// 提取数量
	var amount uint64
	buf := bytes.NewReader(instr.Data[1:9])
	if err := binary.Read(buf, binary.LittleEndian, &amount); err != nil {
		return nil, err
	}

	t := types.InstructionMintTo{
		Mint:   mint,
		To:     to,
		Amount: amount,
	}

	trade = &types.TradeWithPair{
		InstructionMintTo: t,
		Type:              types.TradeTokenMint,
		TxHash:            dtx.TxHash,
	}

	return trade, nil
}

func DecodeInstructionBurn(_ context.Context, _ *svc.ServiceContext, dtx *DecodedTx, instr *solTypes.CompiledInstruction) (trade *types.TradeWithPair, err error) {

	tx := dtx.Tx

	accountIndex := instr.Accounts[0]
	if accountIndex >= len(tx.AccountKeys) {
		return nil, fmt.Errorf("invalid token program index")
	}
	account := tx.AccountKeys[accountIndex]

	// 提取 Mint 地址
	mintIndex := instr.Accounts[1]
	if mintIndex >= len(tx.AccountKeys) {
		return nil, fmt.Errorf("invalid token program index")
	}
	mint := tx.AccountKeys[mintIndex]

	// 提取数量
	var amount uint64
	buf := bytes.NewReader(instr.Data[1:9])
	if err := binary.Read(buf, binary.LittleEndian, &amount); err != nil {
		return nil, err
	}

	burn := types.InstructionBurn{
		Mint:    mint,
		Account: account,
		Amount:  amount,
	}

	trade = &types.TradeWithPair{
		InstructionBurn: burn,
		Type:            types.TradeTokenBurn,
		TxHash:          dtx.TxHash,
	}

	return trade, nil
}
