package token

import (
	"fmt"
	"github.com/blocto/solana-go-sdk/common"
	"github.com/blocto/solana-go-sdk/program/tokenprog"
	"github.com/blocto/solana-go-sdk/types"
	"log"
	"testing"
)

func TestParseMint(t *testing.T) {
	// 示例 Instruction
	instr := types.Instruction{
		ProgramID: common.TokenProgramID,
		Accounts: []types.AccountMeta{
			{PubKey: common.PublicKeyFromString("MintAddressHere"), IsSigner: false, IsWritable: true},
			{PubKey: common.PublicKeyFromString("RecipientAddressHere"), IsSigner: false, IsWritable: true},
			{PubKey: common.PublicKeyFromString("AuthorityAddressHere"), IsSigner: true, IsWritable: false},
		},
		Data: []byte{byte(tokenprog.InstructionMintTo), 0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // 示例数据
	}

	// 解析指令
	param, err := ParseMintToInstruction(instr)
	if err != nil {
		log.Fatalf("Failed to parse instruction: %v", err)
	}

	// 输出解析结果
	fmt.Printf("MintTo Instruction Parsed:\n")
	fmt.Printf("Mint Address: %s\n", param.Mint)
	fmt.Printf("To Address: %s\n", param.To)
	fmt.Printf("Authority Address: %s\n", param.Auth)
	fmt.Printf("Amount: %d\n", param.Amount)
	fmt.Printf("Signers: %v\n", param.Signers)
}
