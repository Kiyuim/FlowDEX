package constants

import "strings"

// IsDevnet reports whether the Solana network in use is devnet.
// It prefers an explicit `network` value (e.g. "devnet"/"mainnet") and
// otherwise falls back to sniffing the RPC node URLs for "devnet" so that
// existing yamls (which already point at devnet endpoints) keep working
// without an explicit Network field.
func IsDevnet(network string, nodeUrls []string) bool {
	if network != "" {
		return strings.EqualFold(strings.TrimSpace(network), "devnet")
	}
	for _, u := range nodeUrls {
		if strings.Contains(strings.ToLower(u), "devnet") {
			return true
		}
	}
	return false
}

const SolChainId = "100000"
const SolChainIdInt = 100000
const TrxChainIdInt = 110000
const EthChainIdInt = 1
const BaseChainIdInt = 8453
const BscChainIdInt = 56
const AtaAccountSize = 165
const MinSolVolume = uint64(1e7)
const AutoSlippage = 2500

const (
	ChainIconEth  = "https://cdn.pumpx.ai/static/img/chain/eth.png"
	ChainIconBsc  = "https://cdn.pumpx.ai/static/img/chain/bsc.png"
	ChainIconBase = "https://cdn.pumpx.ai/static/img/chain/base.png"
	ChainIconSol  = "https://cdn.pumpx.ai/static/img/chain/sol.png"
	ChainIconTron = "https://cdn.pumpx.ai/static/img/chain/trx.png"
)

const SolDecimal = 9

const ProgramStrRaydiumV4 = "CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK"
const ProgramStrRaydiumV2 = "RVKd61ztZW9GUwhRbbLoYVRE5Xf1B2tVscKqwZqXgEr"

const ProgramStrToken = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
const ProgramStrToken2022 = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"
const ProgramStrSystem = "11111111111111111111111111111111"
const ProgramStrAssociatedToken = "ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL"

const ProgramStrPumpFun = "6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P"
const ProgramStrPumpAmm = "pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA"

const TokenStrWrapSol = "So11111111111111111111111111111111111111112"
const TokenStrUSDC = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
const TokenStrUSDT = "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"

const TokenStrWrapTrx = "TNUC9Qb1rRpS5CbWLmNMxXBjyFoydXjWFR"

const SunPumpLogo = "/static/img/brand/sunpump_logo.jpg"
const PumpFunLogo = "/static/img/brand/pump_fun.png"

const TokenStrWTRX = "TNUC9Qb1rRpS5CbWLmNMxXBjyFoydXjWFR"

const (
	TrxContractV2Router  = "TKzxdSv2FZKQrEqkKVgp5DcwEXBEKMg2Ax"
	TrxContractV1Factroy = "TXk8rQSAvPvBBNtqSoY6nCfsXWCSSpTVQF" // JustswapFactory
	TrxContractPsmUsdd   = "TPYmHEhy5n8TCEfYGqW2rPxsghSfzghPDn"
	TrxContractV3Router  = "TQAvWQpT9H916GckwWDJNhYZvQMkuRL7PN"
	TrxContractWtrx      = "TNUC9Qb1rRpS5CbWLmNMxXBjyFoydXjWFR"
)

const WBsc = "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"

const (
	BaseTokenAddressSol  = "solana"
	BaseTokenAddressEth  = "eth"
	BaseTokenAddressBase = "base_eth"
	BaseTokenAddressBsc  = "bnb"
)
const (
	BaseTokenSymbolSol  = "SOL"
	BaseTokenSymbolEth  = "ETH"
	BaseTokenSymbolBase = "BASE_ETH"
	BaseTokenSymbolBsc  = "BNB"
)

const (
	MinWithdrawAmountSol = "0.005"
)

const (
	ChainNameEth  = "eth"
	ChainNameBase = "base"
	ChainNameBsc  = "bsc"
)

const EthDecimal = 18
const BaseDecimal = 18
const BscDecimal = 18

const AllChainIdInt = -1

const EvmChains = "1,8453,56"

const (
	SolQuickBuyDefault  = "0.1,0.2,0.5,1,2,5"
	EthQuickBuyDefault  = "0.01,0.02,0.05,0.1,0.5,1"
	BaseQuickBuyDefault = "0.01,0.02,0.05,0.1,0.5,1"
	BscQuickBuyDefault  = "0.03,0.06,0.15,0.3,1,2"
	QuickSellDefault    = "25,33,50,60,75,100"
)
