package logic

import (
	"context"
	"fmt"
	"os"

	"dex/market/market"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type TokenSecurityCheckLogic struct {
	ctx context.Context
}

func NewTokenSecurityCheckLogic(ctx context.Context) *TokenSecurityCheckLogic {
	return &TokenSecurityCheckLogic{
		ctx: ctx,
	}
}

func (l *TokenSecurityCheckLogic) TokenSecurityCheck(in *market.TokenSecurityCheckRequest) (*market.TokenSecurityCheckResponse, error) {
	// RPC endpoint is env-overridable so the network can be switched without a
	// rebuild; defaults to devnet for the current deployment.
	rpcURL := os.Getenv("MARKET_SOL_RPC_URL")
	if rpcURL == "" {
		rpcURL = "https://api.devnet.solana.com"
	}
	client := rpc.New(rpcURL)
	mint := solana.MustPublicKeyFromBase58(in.MintAddress)
	accountInfo, err := client.GetAccountInfo(context.Background(), mint)
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}
	if accountInfo.Value == nil {
		return nil, fmt.Errorf("account not found")
	}
	data := accountInfo.Value.Data.GetBinary()
	if len(data) < 82 {
		return nil, fmt.Errorf("mint data too short")
	}
	tokenInfo := struct {
		Supply          uint64
		Decimals        uint8
		IsInitialized   bool
		MintAuthority   *solana.PublicKey
		FreezeAuthority *solana.PublicKey
	}{
		Supply:        0,
		Decimals:      0,
		IsInitialized: false,
	}
	offset := 0
	// Mint Authority
	mintAuthOption := uint32(data[offset]) |
		uint32(data[offset+1])<<8 |
		uint32(data[offset+2])<<16 |
		uint32(data[offset+3])<<24
	if mintAuthOption == 1 {
		auth := solana.PublicKeyFromBytes(data[offset+4 : offset+36])
		tokenInfo.MintAuthority = &auth
		offset += 36
	} else {
		offset += 4
	}
	// Supply
	supplyBytes := data[offset : offset+8]
	tokenInfo.Supply = uint64(supplyBytes[0]) |
		uint64(supplyBytes[1])<<8 |
		uint64(supplyBytes[2])<<16 |
		uint64(supplyBytes[3])<<24 |
		uint64(supplyBytes[4])<<32 |
		uint64(supplyBytes[5])<<40 |
		uint64(supplyBytes[6])<<48 |
		uint64(supplyBytes[7])<<56
	offset += 8
	// Decimals
	tokenInfo.Decimals = data[offset]
	offset += 1
	// IsInitialized
	tokenInfo.IsInitialized = data[offset] == 1
	offset += 1
	// Freeze Authority
	freezeAuthOption := uint32(data[offset]) |
		uint32(data[offset+1])<<8 |
		uint32(data[offset+2])<<16 |
		uint32(data[offset+3])<<24
	if freezeAuthOption == 1 {
		freezeAuth := solana.PublicKeyFromBytes(data[offset+4 : offset+36])
		tokenInfo.FreezeAuthority = &freezeAuth
	}
	// Prepare response
	mintAuthStr := ""
	freezeAuthStr := ""
	if tokenInfo.MintAuthority != nil {
		mintAuthStr = tokenInfo.MintAuthority.String()
	}
	if tokenInfo.FreezeAuthority != nil {
		freezeAuthStr = tokenInfo.FreezeAuthority.String()
	}
	mintSafe := tokenInfo.MintAuthority == nil
	freezeSafe := tokenInfo.FreezeAuthority == nil
	var summary string
	if mintSafe && freezeSafe {
		summary = "Safe: No mint or freeze authority."
	} else if !mintSafe && freezeSafe {
		summary = "Mint authority risk: Can be minted."
	} else if mintSafe && !freezeSafe {
		summary = "Freeze authority risk: Can be frozen."
	} else {
		summary = "Mint and freeze authority risk."
	}
	return &market.TokenSecurityCheckResponse{
		MintAddress:         in.MintAddress,
		Supply:              tokenInfo.Supply,
		Decimals:            uint32(tokenInfo.Decimals),
		IsInitialized:       tokenInfo.IsInitialized,
		MintAuthority:       mintAuthStr,
		FreezeAuthority:     freezeAuthStr,
		MintAuthoritySafe:   mintSafe,
		FreezeAuthoritySafe: freezeSafe,
		SecuritySummary:     summary,
	}, nil
}
