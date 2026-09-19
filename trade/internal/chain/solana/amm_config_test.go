package solana

import (
    "testing"
    "github.com/gagliardetto/solana-go"
)

func TestGetAmmConfigPDA(t *testing.T) {
    // 示例参数（请替换为你实际的 programID 和 index）
    programID := solana.MustPublicKeyFromBase58("A1izdbCxDvLjZ2WZFkPdSLNBrrYrhBqxmmzCkm82G4ys")
    index := uint16(7)

    // 期望的 PDA（可从区块浏览器或前端查到）
    expected := "FiyUUSnhBgLhBgVGWNBVozhzSbAbFCU1Q8iWMHH3xUhA"

    pda, err := GetAmmConfigPDA(index, programID)
    if err != nil {
        t.Fatalf("GetAmmConfigPDA error: %v", err)
    }

    if pda.String() != expected {
        t.Errorf("PDA mismatch: got %s, want %s", pda.String(), expected)
    }
}