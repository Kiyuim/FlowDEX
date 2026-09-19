package token

import (
	"context"
	"github.com/gagliardetto/solana-go"
	"testing"
)

func TestGetTokenHolds(t *testing.T) {
	holders, _ := GetTokenHolds(context.Background(),
		"https://mainnet.helius-rpc.com?api-key=c9a7aa2a-c8b6-4720-87e0-aa975aba56b7",
		solana.MustPublicKeyFromBase58("A8C3xuqscfmyLrte3VmTqrAq8kgMASius9AFNANwpump"))
	t.Log(len(holders))
}
