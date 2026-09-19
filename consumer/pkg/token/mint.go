package token

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/blocto/solana-go-sdk/common"
	"github.com/blocto/solana-go-sdk/program/tokenprog"
	"github.com/blocto/solana-go-sdk/types"
)

type MintToParam struct {
	Mint    common.PublicKey
	To      common.PublicKey
	Auth    common.PublicKey
	Signers []common.PublicKey
	Amount  uint64
}

func ParseMintToInstruction(instr types.Instruction) (MintToParam, error) {
	// 验证 ProgramID 是否为 SPL Token Program
	if instr.ProgramID != common.TokenProgramID {
		return MintToParam{}, fmt.Errorf("not a valid SPL Token Program instruction")
	}

	// 解析 Data 字段
	data := instr.Data
	if len(data) < 9 {
		return MintToParam{}, fmt.Errorf("invalid data length")
	}

	// 读取指令类型和金额
	var (
		parsedInstr uint8
		amount      uint64
	)
	buf := bytes.NewReader(data)
	if err := binary.Read(buf, binary.LittleEndian, &parsedInstr); err != nil {
		return MintToParam{}, err
	}
	if tokenprog.Instruction(parsedInstr) != tokenprog.InstructionMintTo {
		return MintToParam{}, fmt.Errorf("not a MintTo instruction")
	}
	if err := binary.Read(buf, binary.LittleEndian, &amount); err != nil {
		return MintToParam{}, err
	}

	// 解析 Accounts 字段
	if len(instr.Accounts) < 3 {
		return MintToParam{}, fmt.Errorf("invalid accounts length")
	}

	mint := instr.Accounts[0].PubKey
	to := instr.Accounts[1].PubKey
	auth := instr.Accounts[2].PubKey

	// 获取 Signers
	signers := make([]common.PublicKey, 0, len(instr.Accounts)-3)
	for _, acc := range instr.Accounts[3:] {
		signers = append(signers, acc.PubKey)
	}

	// 返回解析结果
	return MintToParam{
		Mint:    mint,
		To:      to,
		Auth:    auth,
		Signers: signers,
		Amount:  amount,
	}, nil
}
