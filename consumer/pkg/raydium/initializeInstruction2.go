package raydium

import (
	"github.com/blocto/solana-go-sdk/types"
	"github.com/near/borsh-go"
)

type InitializeInstruction2 struct {
	Nonce          uint8  `borsh_skip:"false"` // nonce used to create valid program address
	OpenTime       uint64 // UTC timestamp for pool open
	InitPcAmount   uint64 // Initial token pc amount
	InitCoinAmount uint64 // Initial token coin amount
}

// ToBytes Encode to borsh bytes
func (ii *InitializeInstruction2) ToBytes() ([]byte, error) {
	return borsh.Serialize(*ii)
}

// FromBytes Decode from borsh bytes
func (ii *InitializeInstruction2) FromBytes(data []byte) error {
	return borsh.Deserialize(ii, data)
}

func DecodeRaydiumCreate(instruction *types.CompiledInstruction) (*InitializeInstruction2, error) {

	ii := &InitializeInstruction2{}
	err := ii.FromBytes(instruction.Data[1:])
	if err != nil {
		return nil, err
	}
	return ii, nil
}
