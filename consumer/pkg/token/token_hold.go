package token

import (
	"context"
	"dex/pkg/transfer"
	"log"

	"github.com/blocto/solana-go-sdk/rpc"
	"github.com/davecgh/go-spew/spew"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/gagliardetto/solana-go"
)

func GetTokenHolds(ctx context.Context, heliusEndpoint string, tokenAddress solana.PublicKey) ([]*ProgramAccount, error) {

	solClient := rpc.New(rpc.WithEndpoint(heliusEndpoint))

	// 获取所有的 Token 账户
	config := rpc.GetTokenAccountsByOwnerConfigFilter{
		ProgramId: solana.TokenProgramID.String(),
		// ProgramId: "2FCSMaMfw8YXjuPE4QfLikAqXkNXrVwsppU8GQdSXcLN",
	}

	ownerConfig := rpc.GetTokenAccountsByOwnerConfig{
		Commitment: rpc.CommitmentConfirmed,
		Encoding:   rpc.AccountEncodingJsonParsed,
	}
	result, err := solClient.GetTokenAccountsByOwnerWithConfig(
		context.Background(),
		"2FCSMaMfw8YXjuPE4QfLikAqXkNXrVwsppU8GQdSXcLN", config, ownerConfig)
	if err != nil {
		log.Fatalf("获取账户失败: %v", err)
	}

	accounts := slice.FilterMap[rpc.GetProgramAccount, *ProgramAccount](result.Result.Value, func(_ int, account rpc.GetProgramAccount) (*ProgramAccount, bool) {
		programAccount, err := transfer.Map2Struct[any, *ProgramAccount](account.Account.Data.(map[string]any))
		programAccount.Parsed.Info.AccountAddress = account.Pubkey
		if err != nil {
			return nil, false
		}
		uiAmount := programAccount.Parsed.Info.TokenAmount.UiAmount
		// 筛选持有大于 1e-9 的账户
		if uiAmount > 0 {
			return programAccount, true
		}
		return nil, false
	})

	// 输出结果
	for _, account := range accounts {
		spew.Dump(account)
	}

	return accounts, nil
}
