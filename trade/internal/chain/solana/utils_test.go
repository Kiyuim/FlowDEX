package solana

import (
	"context"
	"testing"
	"time"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/blocto/solana-go-sdk/common"
	"github.com/blocto/solana-go-sdk/program/address_lookup_table"
	"github.com/blocto/solana-go-sdk/rpc"
	"github.com/blocto/solana-go-sdk/types"
)

func TestCreateGlobalALT(t *testing.T) {
	ctx := context.Background()
	// Create a dummy payer (for test only, do not use in production)
	payer := types.NewAccount()
	// Use a fake recent slot for test; in real use, get from RPC
	recentSlot := uint64(12345678)

	inst := CreateGlobalALT(ctx, payer, recentSlot)
	// if inst.ProgramID == types.PublicKey{} {
	// 	t.Fatal("CreateGlobalALT returned instruction with empty ProgramID")
	// }
	t.Logf("CreateGlobalALT instruction: %+v", inst)
}

func TestCreateGlobalALT_Devnet(t *testing.T) {
	ctx := context.Background()

	// 1. 使用提供的devnet私钥
	priv := "5RQifBbA1Hb9C6foL1Qpzxzmt87Gtv8UYt2LHQfmhCLeYQ4NjKnc9N5F6uZMmgNE8ojJw79zPjcFT5enjcntHtoZ"
	payer, err := types.AccountFromBase58(priv)
	if err != nil {
		t.Fatalf("failed to parse devnet private key: %v", err)
	}

	// 2. 连接devnet
	c := client.NewClient(rpc.DevnetRPCEndpoint)

	// 获取链上最新slot，必须用真实slot，否则ALT创建会失败
	slot, err := c.RpcClient.GetSlotWithConfig(ctx, rpc.GetSlotConfig{
		Commitment: rpc.CommitmentFinalized,
	})
	if err != nil {
		t.Fatalf("failed to get slot: %v", err)
	}

	// 3. 推导 ALT PDA 地址
	seeds := [][]byte{
		payer.PublicKey.Bytes(),
		make([]byte, 8), // slot (little endian)
	}
	for i := uint(0); i < 8; i++ {
		seeds[1][i] = byte(slot.Result >> (8 * i))
	}
	altProgramID := common.AddressLookupTableProgramID
	altPDA, bump, err := common.FindProgramAddress(seeds, altProgramID)
	if err != nil {
		t.Fatalf("failed to derive ALT PDA: %v", err)
	}

	// 4. 构造ALT创建指令，传入 PDA 和 bump
	inst := address_lookup_table.CreateLookupTable(address_lookup_table.CreateLookupTableParams{
		LookupTable: altPDA,
		Authority:   payer.PublicKey,
		Payer:       payer.PublicKey,
		RecentSlot:  slot.Result,
		BumpSeed:    bump,
	})

	t.Logf("ALT PDA: %s", altPDA.String())

	// 3. 获取最新slot和blockhash
	block, err := c.GetLatestBlockhash(ctx)
	if err != nil {
		t.Fatalf("failed to get latest blockhash: %v", err)
	}

	// 5. 构造并签名交易
	tx, err := types.NewTransaction(types.NewTransactionParam{
		Message: types.NewMessage(types.NewMessageParam{
			FeePayer:        payer.PublicKey,
			Instructions:    []types.Instruction{inst},
			RecentBlockhash: block.Blockhash,
		}),
		Signers: []types.Account{payer},
	})
	if err != nil {
		t.Fatalf("failed to build transaction: %v", err)
	}

	// 6. 发送交易
	sig, err := c.SendTransaction(ctx, tx)
	if err != nil {
		t.Fatalf("failed to send transaction: %v", err)
	}
	t.Logf("ALT creation tx signature: %s", sig)

	// 7. 等待确认
	t.Log("Waiting 20s for confirmation...")
	time.Sleep(20 * time.Second)
}

func TestExtendGlobalALT_Devnet(t *testing.T) {
	ctx := context.Background()

	// 1. 使用同一个devnet私钥
	priv := "5RQifBbA1Hb9C6foL1Qpzxzmt87Gtv8UYt2LHQfmhCLeYQ4NjKnc9N5F6uZMmgNE8ojJw79zPjcFT5enjcntHtoZ"
	payer, err := types.AccountFromBase58(priv)
	if err != nil {
		t.Fatalf("failed to parse devnet private key: %v", err)
	}

	c := client.NewClient(rpc.DevnetRPCEndpoint)

	altPDA := common.PublicKeyFromString("Dqgo35VeFKqzqJteZpDPtTLCFEknicbhfEXWeMYoXB7m")

	// 2. 构造要添加的固定地址
	addresses := []common.PublicKey{
		common.PublicKeyFromString("11111111111111111111111111111111"),
		common.PublicKeyFromString("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"),
		common.PublicKeyFromString("AToknGPvoterZz7p6h1p5Qd6hw2yffA9H9n1t6"),
		common.PublicKeyFromString("SysvarRent111111111111111111111111111111111"),
		common.PublicKeyFromString("metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s"),
	}

	// 3. 构造扩展指令
	extendInst := address_lookup_table.ExtendLookupTable(address_lookup_table.ExtendLookupTableParams{
		LookupTable: altPDA,
		Authority:   payer.PublicKey,
		Payer:       &payer.PublicKey,
		Addresses:   addresses,
	})

	// 4. 获取最新blockhash
	block, err := c.GetLatestBlockhash(ctx)
	if err != nil {
		t.Fatalf("failed to get latest blockhash: %v", err)
	}

	// 5. 构造并签名交易
	tx, err := types.NewTransaction(types.NewTransactionParam{
		Message: types.NewMessage(types.NewMessageParam{
			FeePayer:        payer.PublicKey,
			Instructions:    []types.Instruction{extendInst},
			RecentBlockhash: block.Blockhash,
		}),
		Signers: []types.Account{payer},
	})
	if err != nil {
		t.Fatalf("failed to build transaction: %v", err)
	}

	// 6. 发送交易
	sig, err := c.SendTransaction(ctx, tx)
	if err != nil {
		t.Fatalf("failed to send extend transaction: %v", err)
	}
	t.Logf("ALT extend tx signature: %s", sig)
	t.Logf("ALT PDA: %s", altPDA.String())

	// 7. 等待确认
	t.Log("Waiting 20s for confirmation...")
	time.Sleep(20 * time.Second)
}
