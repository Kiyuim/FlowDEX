package block

import (
	"testing"
)

// Real SwapEvent "Program data:" payload from a devnet buy tx seeded by devnet_seed.js
// (tx 28DzX2Vc...). Validates that the decoder parses the deployed contract's event.
const realMeteoraSwapLog = "Program data: QMbN6CYIceJ1re8RRzetlcApmEDhtt3VQotp5zQTPLCRiluHXDl6/EjZLOfPMwtzK4tZRd6NKipGnTLODfJc+i7Yu/8ip0uHH/qt3rp5HZ8oNlRrGNkU5R9YBAkSfQdFU4KPJuKkGx7AHy4BAAAAAAAAAAAAAAAAAKcA+sakAAAAwB8uAQAAAABZd8s0rdACAA=="

func TestDecodeMeteoraSwapEvent(t *testing.T) {
	var ev MeteoraSwapEvent
	ok, err := decodeMeteoraEvent([]string{realMeteoraSwapLog}, MeteoraEventSwap, &ev)
	if err != nil {
		t.Fatalf("decode err: %v", err)
	}
	if !ok {
		t.Fatal("SwapEvent not found / discriminator mismatch")
	}
	// values verified independently against the on-chain tx
	if ev.Direction != 0 {
		t.Errorf("Direction = %d, want 0 (buy)", ev.Direction)
	}
	if ev.AmountIn != 19800000 {
		t.Errorf("AmountIn = %d, want 19800000", ev.AmountIn)
	}
	if ev.AmountOut != 707712909479 {
		t.Errorf("AmountOut = %d, want 707712909479", ev.AmountOut)
	}
	if ev.RealSolReserves != 19800000 {
		t.Errorf("RealSolReserves = %d, want 19800000", ev.RealSolReserves)
	}
	if ev.RealTokenReserves != 792392287090521 {
		t.Errorf("RealTokenReserves = %d, want 792392287090521", ev.RealTokenReserves)
	}
	if ev.Mint.String() == "" || ev.BondingCurve.String() == "" {
		t.Error("mint/bonding_curve not parsed")
	}
	t.Logf("OK mint=%s curve=%s user=%s", ev.Mint, ev.BondingCurve, ev.User)
}
