package token

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/blocto/solana-go-sdk/common"
	"github.com/blocto/solana-go-sdk/program/token"
	"github.com/blocto/solana-go-sdk/rpc"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/gagliardetto/solana-go"
	"net/http"
	"time"
)

// TokenAmount 表示 token 的数量和精度信息
type TokenAmount struct {
	Amount         string  `json:"amount"`
	Decimals       int     `json:"decimals"`
	UiAmount       float64 `json:"uiAmount"`
	UiAmountString string  `json:"uiAmountString"`
}

// AccountInfo 表示账户的详细信息
type AccountInfo struct {
	IsNative       bool        `json:"isNative"`
	Mint           string      `json:"mint"`
	Owner          string      `json:"owner"`
	AccountAddress string      `json:"accountAddress"`
	State          string      `json:"state"`
	TokenAmount    TokenAmount `json:"tokenAmount"`
}

// ParsedAccount 表示解析的账户信息
type ParsedAccount struct {
	Info AccountInfo `json:"info"`
	Type string      `json:"type"`
}

// ProgramAccount 表示完整的账户数据结构
type ProgramAccount struct {
	Parsed  ParsedAccount `json:"parsed"`
	Program string        `json:"program"`
	Space   int           `json:"space"`
}

func GetTokenHolders(ctx context.Context, heliusEndpoint string, tokenAddress solana.PublicKey) ([]*ProgramAccount, error) {
	filters := []rpc.GetProgramAccountsConfigFilter{
		{
			DataSize: token.TokenAccountSize,
		},
		{
			MemCmp: &rpc.GetProgramAccountsConfigFilterMemCmp{
				Offset: 0,
				Bytes:  tokenAddress.String(),
			},
		},
	}

	httpClient := &http.Client{Timeout: 300 * time.Second}

	solClient := rpc.New(rpc.WithEndpoint(heliusEndpoint), rpc.WithHTTPClient(httpClient))

	var allAccounts []*ProgramAccount

	// dataSlice := &rpc.DataSlice{
	// 	Offset: offset,
	// 	Length: 165, // 每次请求的大小，通过参数控制
	// }

	res, err := solClient.GetProgramAccountsWithConfig(
		ctx,
		solana.TokenProgramID.String(),
		rpc.GetProgramAccountsConfig{
			Filters:  filters,
			Encoding: rpc.AccountEncodingBase64,
			// DataSlice:  dataSlice,
			Commitment: rpc.CommitmentConfirmed,
		},
	)

	if err != nil {
		return nil, err
	}
	accounts := slice.FilterMap[rpc.GetProgramAccount, *ProgramAccount](res.Result, func(_ int, account rpc.GetProgramAccount) (*ProgramAccount, bool) {
		segs, ok := account.Account.Data.([]any)
		if !ok {
			return nil, false
		}

		decodedData, err := base64.StdEncoding.DecodeString(segs[0].(string))
		if err != nil {
			return nil, false
		}

		tokenAccount, err := token.DeserializeTokenAccount(decodedData, common.TokenProgramID)
		if err != nil {
			return nil, false
		}

		if tokenAccount.Amount > 0 {
			account := &ProgramAccount{
				Parsed: ParsedAccount{
					Info: AccountInfo{
						Mint:           tokenAccount.Mint.String(),
						Owner:          tokenAccount.Owner.String(),
						AccountAddress: account.Pubkey,
						TokenAmount: TokenAmount{
							Amount: fmt.Sprintf("%v", tokenAccount.Amount),
						},
					},
				},
			}
			return account, true
		}
		return nil, false
	})

	// 将获取到的账户追加到整体结果
	allAccounts = append(allAccounts, accounts...)
	return allAccounts, nil
}

func GetToken2022Holders(ctx context.Context, heliusEndpoint string, tokenAddress solana.PublicKey) ([]*ProgramAccount, error) {

	filters := []rpc.GetProgramAccountsConfigFilter{
		{
			MemCmp: &rpc.GetProgramAccountsConfigFilterMemCmp{
				Offset: 0,
				Bytes:  tokenAddress.String(),
			},
		},
		{
			MemCmp: &rpc.GetProgramAccountsConfigFilterMemCmp{
				Offset: 165,
				Bytes:  "3",
			},
		},
	}

	httpClient := &http.Client{Timeout: 180 * time.Second}

	solClient := rpc.New(rpc.WithEndpoint(heliusEndpoint), rpc.WithHTTPClient(httpClient))

	var allAccounts []*ProgramAccount

	res, err := solClient.GetProgramAccountsWithConfig(
		ctx,
		solana.Token2022ProgramID.String(),
		rpc.GetProgramAccountsConfig{
			Filters:  filters,
			Encoding: rpc.AccountEncodingBase64,
			// DataSlice:  dataSlice,
			Commitment: rpc.CommitmentConfirmed,
		},
	)

	if err != nil {
		return nil, err
	}
	accounts := slice.FilterMap[rpc.GetProgramAccount, *ProgramAccount](res.Result, func(_ int, account rpc.GetProgramAccount) (*ProgramAccount, bool) {
		segs, ok := account.Account.Data.([]any)
		if !ok {
			return nil, false
		}

		decodedData, err := base64.StdEncoding.DecodeString(segs[0].(string))
		if err != nil {
			return nil, false
		}

		tokenAccount, err := token.TokenAccountFromData(decodedData[:165])
		if err != nil {
			return nil, false
		}

		if tokenAccount.Amount > 0 {
			account := &ProgramAccount{
				Parsed: ParsedAccount{
					Info: AccountInfo{
						Mint:           tokenAccount.Mint.String(),
						Owner:          tokenAccount.Owner.String(),
						AccountAddress: account.Pubkey,
						TokenAmount: TokenAmount{
							Amount: fmt.Sprintf("%v", tokenAccount.Amount),
						},
					},
				},
			}
			return account, true
		}
		return nil, false
	})

	// 将获取到的账户追加到整体结果
	allAccounts = append(allAccounts, accounts...)
	return allAccounts, nil
}
