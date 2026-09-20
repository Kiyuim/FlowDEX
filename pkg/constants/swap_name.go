package constants

const (
	PumpFun                      = "PumpFun"
	PumpMeteora                  = "PumpMeteora"
	PumpMeteoraV2                = "PumpMeteoraV2" // optimized/hardened pump-meteora build
	PumpSwap                     = "PumpSwap"
	RaydiumV4                    = "RaydiumV4"
	RaydiumConcentratedLiquidity = "RaydiumClmm"
	RaydiumCPMM                  = "RaydiumCpmm"
	PancakeSwapV3                = "PancakeSwap V3"
	PancakeSwapV2                = "PancakeSwap V2"
	UniswapV3                    = "Uniswap V3"
	UniswapV2                    = "Uniswap V2"
	FourMemeV2                   = "FourMeme V2"
)

// swap name
const (
	SwapNameUniv2 = "Uniswap V2"
	SwapNameUniv3 = "Uniswap V3"

	SwapNamePancakeV2 = "PancakeSwap V2"
	SwapNamePancakeV3 = "PancakeSwap V3"

	SwapNameFourMemeV2 = "FourMeme V2"
)

// BondingCurveSources are the launchpad "token source" programs that trade on an
// on-chain bonding curve (as opposed to an AMM pair). New token creations, the
// completing/graduating lists, and the bonding-curve liquidity formula all apply to
// every source in this set.
var BondingCurveSources = []string{PumpFun, PumpMeteora, PumpMeteoraV2}

// IsBondingCurveSource reports whether a pair/trade source name is a bonding-curve
// launchpad (pump.fun or either pump-meteora build).
func IsBondingCurveSource(name string) bool {
	return name == PumpFun || name == PumpMeteora || name == PumpMeteoraV2
}
