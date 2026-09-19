
ç™
market/market.protomarket"á
Kline
chain_id (RchainId
interval (	Rinterval
	pair_addr (	RpairAddr
open (Ropen
high (Rhigh
low (Rlow
close (Rclose
	mcap_open (RmcapOpen
	mcap_high	 (RmcapHigh
mcap_low
 (RmcapLow

mcap_close (R	mcapClose

amount_usd (R	amountUsd!
volume_token (RvolumeToken
candle_time (R
candleTime
	buy_count (RbuyCount

sell_count (R	sellCount
total_count (R
totalCount
close_at (RcloseAt
open_at (RopenAt
	avg_price (RavgPrice
mkt_cap (RmktCap

pump_point (R	pumpPoint

token_addr (	R	tokenAddr"g
Klines!
list (2.market.KlineRlist:
fluctuation (2.market.KlineFluctuationRfluctuation"ˆ
KlineFluctuation0
kline_fluctuation_1m (RklineFluctuation1m0
kline_fluctuation_5m (RklineFluctuation5m2
kline_fluctuation_15m (RklineFluctuation15m0
kline_fluctuation_1h (RklineFluctuation1h0
kline_fluctuation_4h (RklineFluctuation4h2
kline_fluctuation_12h (RklineFluctuation12h2
kline_fluctuation_24h (RklineFluctuation24h"`
Kline24InfoItem
	change_24 (Rchange24
txs_24h (Rtxs24h
vol_24h (Rvol24h"º
KlineUp
chain (	Rchain!
pair_address (	RpairAddress
	timestamp (R	timestamp
interval (	Rinterval
open (	Ropen
high (	Rhigh
low (	Rlow
close (	Rclose
volume (	Rvolume
swaps	 (Rswaps
amount
 (	Ramount
buys (Rbuys

buy_amount (	R	buyAmount

buy_volume (	R	buyVolume
sells (Rsells
sell_amount (	R
sellAmount
sell_volume (	R
sellVolume"„
Trade!
pair_address (	RpairAddress
tx_hash (	RtxHash
maker (	Rmaker
to (	Rto

trade_type (R	tradeType*
base_token_amount (RbaseTokenAmount!
token_amount (RtokenAmount/
base_token_price_usd (RbaseTokenPriceUsd
	total_usd	 (RtotalUsd&
token_price_usd
 (RtokenPriceUsd
	block_num (RblockNum'
block_timestamp (RblockTimestamp

clamp_type (R	clampType
	swap_name (	RswapName
sort (Rsort"+
Trades!
list (2.market.TradeRlist"N
SearchPairByAddrRequest
chain_id (RchainId
address (	Raddress"[
QueryUserHoldersRequest
address (	Raddress
page (Rpage
size (Rsize"\
QueryPairContractSafeRequest
chain_id (RchainId!
pair_address (	RpairAddress"
GetConfigRequest"
GetConfigResponse"
GetBannerConfigRequest"
GetBannerConfigResponse"
GetUpdateConfigRequest"
GetUpdateConfigResponse"§
GetTokenListRequest
chain_id (RchainId
page_no (RpageNo
	page_size (RpageSize
sorted_type (	R
sortedType'
honeypot_filter (	RhoneypotFilter
price_order (	R
priceOrder!
change_order (	RchangeOrder.
honeypot_non_filter (	RhoneypotNonFilter"W
GetTokenListResponse)
list (2.market.TokenInfoItemRlist
total (Rtotal"Œ
TokenInfoItem
chain_id (RchainId

chain_icon (	R	chainIcon#
token_address (	RtokenAddress

token_icon (	R	tokenIcon!
token_symbol (	RtokenSymbol
token_price (R
tokenPrice
change (Rchange
mkt_cap (RmktCap!
pair_address	 (	RpairAddress
txs_24h
 (Rtxs24h
vol_24h (Rvol24h

hold_count (R	holdCount
change24 (Rchange24)
twitter_username (	RtwitterUsername
website (	Rwebsite

top_holder (R	topHolder
	liquidity (R	liquidity
	freezable (R	freezable
mintable (Rmintable"’
GetPumpTokenListRequest
chain_id (RchainId
pump_status (R
pumpStatus
sorted_type (	R
sortedType'
honeypot_filter (	RhoneypotFilter
page_no (RpageNo
	page_size (RpageSize"[
GetPumpTokenListResponse)
list (2.market.PumpTokenItemRlist
total (Rtotal"Í
PumpTokenItem
chain_id (RchainId

chain_icon (	R	chainIcon#
token_address (	RtokenAddress

token_icon (	R	tokenIcon

token_name (	R	tokenName
launch_time (R
launchTime
mkt_cap (RmktCap

hold_count (R	holdCount
txs_24h	 (Rtxs24h
vol_24h
 (Rvol24h+
domestic_progress (RdomesticProgress)
twitter_username (	RtwitterUsername
telegram (	Rtelegram
change24 (Rchange24!
pair_address (	RpairAddress"≠
GetClmmPoolListRequest
chain_id (RchainId!
pool_version (RpoolVersion
sorted_type (	R
sortedType
page_no (RpageNo
	page_size (RpageSize"Y
GetClmmPoolListResponse(
list (2.market.ClmmPoolItemRlist
total (Rtotal"ƒ
ClmmPoolItem
chain_id (RchainId

chain_icon (	R	chainIcon

pool_state (	R	poolState(
input_vault_mint (	RinputVaultMint*
output_vault_mint (	RoutputVaultMint,
input_token_symbol (	RinputTokenSymbol.
output_token_symbol (	RoutputTokenSymbol(
input_token_icon (	RinputTokenIcon*
output_token_icon	 (	RoutputTokenIcon$
trade_fee_rate
 (RtradeFeeRate
launch_time (R
launchTime#
liquidity_usd (RliquidityUsd
txs_24h (Rtxs24h
vol_24h (Rvol24h
apr (Rapr!
pool_version (RpoolVersion"N
GetPairInfoByTokensResponse/

token_info (2.market.PairInfoR	tokenInfo"∫
PairInfo
chain_id (RchainId
address (	Raddress
name (	Rname'
factory_address (	RfactoryAddress,
base_token_address (	RbaseTokenAddress#
token_address (	RtokenAddress*
base_token_symbol (	RbaseTokenSymbol!
token_symbol (	RtokenSymbol,
base_token_decimal	 (RbaseTokenDecimal#
token_decimal
 (RtokenDecimal:
base_token_is_native_token (RbaseTokenIsNativeToken/
base_token_is_token0 (RbaseTokenIsToken03
init_base_token_amount (RinitBaseTokenAmount*
init_token_amount (RinitTokenAmount9
current_base_token_amount (RcurrentBaseTokenAmount0
current_token_amount (RcurrentTokenAmount
fdv (Rfdv
mkt_cap (RmktCap
token_price (R
tokenPrice(
base_token_price (RbaseTokenPrice
	block_num (RblockNum

block_time (R	blockTime.
highest_token_price (RhighestTokenPrice*
latest_trade_time (RlatestTradeTime"F
SearchTokenPairRequest
type (	Rtype
content (	Rcontent"
SearchTokenPairResponse"9
GetMarketInfoRequest!
pair_address (	RpairAddress"
GetMarketInfoResponse"R
GetPairInfoRequest
chain_id (RchainId!
pair_address (	RpairAddress"≈
GetPairInfoResponse
chain_id (RchainId
address (	Raddress
name (	Rname'
factory_address (	RfactoryAddress,
base_token_address (	RbaseTokenAddress#
token_address (	RtokenAddress*
base_token_symbol (	RbaseTokenSymbol!
token_symbol (	RtokenSymbol,
base_token_decimal	 (RbaseTokenDecimal#
token_decimal
 (RtokenDecimal:
base_token_is_native_token (RbaseTokenIsNativeToken/
base_token_is_token0 (RbaseTokenIsToken03
init_base_token_amount (RinitBaseTokenAmount*
init_token_amount (RinitTokenAmount9
current_base_token_amount (RcurrentBaseTokenAmount0
current_token_amount (RcurrentTokenAmount
fdv (Rfdv
mkt_cap (RmktCap
token_price (R
tokenPrice(
base_token_price (RbaseTokenPrice
	block_num (RblockNum

block_time (R	blockTime.
highest_token_price (RhighestTokenPrice*
latest_trade_time (RlatestTradeTime"=
GetHoldTokenUsersRequest!
pair_address (	RpairAddress"
GetHoldTokenUsersResponse"Õ
GetLatestTradeInfoRequest
chain_id (RchainId!
pair_address (	RpairAddress
page_no (RpageNo
	page_size (RpageSize
fixed (Rfixed
start (Rstart
end (Rend"U
GetLatestTradeInfoResponse
total (Rtotal!
list (2.market.TradeRlist"=
CheckPairInfoRiskRequest!
pair_address (	RpairAddress"
CheckPairInfoRiskResponse"Û
GetKlineRequest
chain_id (RchainId!
pair_address (	RpairAddress
interval (	Rinterval%
from_timestamp (RfromTimestamp!
to_timestamp (RtoTimestamp
limit (Rlimit&
must_count_back (RmustCountBack"8
GetKlineResponse$
list (2.market.NewKlineRlist"û
NewKline
open (Ropen
high (Rhigh
low (Rlow
close (Rclose!
volume_token (RvolumeToken
candle_time (R
candleTime"7
GetKlineTagRequest!
pair_address (	RpairAddress"
GetKlineTagResponse"k
GetWalletTradeInfoByPairRequest!
pair_address (	RpairAddress%
wallet_address (	RwalletAddress""
 GetWalletTradeInfoByPairResponse"
AuditTokenRequest"
AuditTokenResponse"
GetKOLPlanRequest"
GetKOLPlanResponse"W
GetAmmPoolByPairRequest
chain_id (RchainId!
pair_address (	RpairAddress"Î
GetAmmPoolByPairResponse
chain_id (RchainId!
pair_address (	RpairAddressB
raydium_amm_keys (2.market.RaydiumAmmKeysH RraydiumAmmKeys9
pump_fun_keys (2.market.PumpFunKeysH RpumpFunKeysB
pool_information"T
GetPoolByPairRequest
chain_id (RchainId!
pair_address (	RpairAddress"Ø
GetPoolByPairResponse
chain_id (RchainId!
pair_address (	RpairAddressB
raydium_amm_keys (2.market.RaydiumAmmKeysH RraydiumAmmKeys9
pump_fun_keys (2.market.PumpFunKeysH RpumpFunKeysE
raydium_clmm_keys (2.market.RaydiumClmmKeysH RraydiumClmmKeysB
pool_information"”
RaydiumAmmKeys
amm_id (	RammId#
amm_authority (	RammAuthority&
amm_open_orders (	RammOpenOrders*
amm_target_orders (	RammTargetOrders5
pool_coin_token_account (	RpoolCoinTokenAccount1
pool_pc_token_account (	RpoolPcTokenAccount(
serum_program_id (	RserumProgramId!
serum_market (	RserumMarket

serum_bids	 (	R	serumBids

serum_asks
 (	R	serumAsks*
serum_event_queue (	RserumEventQueue7
serum_coin_vault_account (	RserumCoinVaultAccount3
serum_pc_vault_account (	RserumPcVaultAccount,
serum_vault_signer (	RserumVaultSigner
	create_at (	RcreateAt
	update_at (	RupdateAt
	base_mint (	RbaseMint

quote_mint (	R	quoteMint"
PumpFunKeys"·
RaydiumClmmKeys

amm_config (	R	ammConfig

pool_state (	R	poolState
input_vault (	R
inputVault!
output_vault (	RoutputVault+
observation_state (	RobservationState#
token_program (	RtokenProgram,
token_program_2022 (	RtokenProgram2022!
memo_program (	RmemoProgram(
input_vault_mint	 (	RinputVaultMint*
output_vault_mint
 (	RoutputVaultMint-
remaining_accounts (	RremainingAccounts$
trade_fee_rate (RtradeFeeRate"U
GetTokenInfoRequest
chain_id (RchainId#
token_address (	RtokenAddress"Y
GetTokenInfoByPairRequest
chain_id (RchainId!
pair_address (	RpairAddress"∆
GetTokenInfoResponse
chain_id (RchainId
address (	Raddress
name (	Rname
symbol (	Rsymbol
decimals (Rdecimals!
total_supply (RtotalSupply
icon (	Ricon

hold_count (R	holdCount'
is_ca_drop_owner	 (RisCaDropOwner 
is_ca_verify
 (R
isCaVerify"
is_honey_scam (RisHoneyScam$
is_liquid_lock (RisLiquidLock+
is_can_pause_trade (RisCanPauseTrade)
is_can_change_tax (RisCanChangeTax+
is_have_black_list (RisHaveBlackList%
is_can_all_sell (RisCanAllSell"
is_have_proxy (RisHaveProxy/
is_can_external_call (RisCanExternalCall'
is_can_add_token (RisCanAddToken-
is_can_change_token (RisCanChangeToken
sell_tax (RsellTax
buy_tax (RbuyTax)
twitter_username (	RtwitterUsername
website (	Rwebsite
telegram (	Rtelegram
is_check_ca (R	isCheckCa
check_ca_at (R	checkCaAt
program (	Rprogram"[
GetPairInfoByTokenRequest
chain_id (RchainId#
token_address (	RtokenAddress"Ã
GetPairInfoByTokenResponse
chain_id (RchainId
address (	Raddress
name (	Rname'
factory_address (	RfactoryAddress,
base_token_address (	RbaseTokenAddress#
token_address (	RtokenAddress*
base_token_symbol (	RbaseTokenSymbol!
token_symbol (	RtokenSymbol,
base_token_decimal	 (RbaseTokenDecimal#
token_decimal
 (RtokenDecimal:
base_token_is_native_token (RbaseTokenIsNativeToken/
base_token_is_token0 (RbaseTokenIsToken03
init_base_token_amount (RinitBaseTokenAmount*
init_token_amount (RinitTokenAmount9
current_base_token_amount (RcurrentBaseTokenAmount0
current_token_amount (RcurrentTokenAmount
fdv (Rfdv
mkt_cap (RmktCap
token_price (R
tokenPrice(
base_token_price (RbaseTokenPrice
	block_num (RblockNum

block_time (R	blockTime.
highest_token_price (RhighestTokenPrice*
latest_trade_time (RlatestTradeTime"[
GetTokenMarketInfoRequest
chain_id (RchainId#
token_address (	RtokenAddress" 

GetTokenMarketInfoResponse
chain_id (RchainId

chain_icon (	R	chainIcon#
token_address (	RtokenAddress!
pair_address (	RpairAddress!
token_symbol (	RtokenSymbol
decimals (Rdecimals!
total_supply (RtotalSupply

token_icon (	R	tokenIcon

hold_count	 (R	holdCount
dex_name
 (	RdexName)
twitter_username (	RtwitterUsername
website (	Rwebsite
telegram (	Rtelegram3
init_base_token_amount (RinitBaseTokenAmount*
init_token_amount (RinitTokenAmount9
current_base_token_amount (RcurrentBaseTokenAmount0
current_token_amount (RcurrentTokenAmount
fdv (Rfdv
mkt_cap (RmktCap
sell_tax (RsellTax
buy_tax (RbuyTax
is_check_ca (R	isCheckCa
token_price (R
tokenPrice(
base_token_price (RbaseTokenPrice
	change_24 (Rchange24
txs_24h (Rtxs24h
vol_24h (Rvol24h

created_at (R	createdAt
is_followed (R
isFollowed
	liquidity (R	liquidity%
lock_liquidity  (RlockLiquidity
	freezable! (R	freezable
mintable" (Rmintable*
base_token_symbol# (	RbaseTokenSymbol+
domestic_progress$ (RdomesticProgress
	change_1m% (Rchange1m
	change_5m& (Rchange5m
	change_1h' (Rchange1h
ai_tag( (RaiTag!
ai_narrative) (	RaiNarrative"ë
GetTokenHolderListRequest
chain_id (RchainId#
token_address (	RtokenAddress
page_no (RpageNo
	page_size (RpageSize"f
GetTokenHolderListResponse
total (Rtotal2
list (2.market.GetTokenHolderListItemRlist"w
GetTokenHolderListItem

proportion (R
proportion
balance (Rbalance#
owner_address (	RownerAddress"y
FavoriteTokenRequest
chain_id (RchainId!
pair_address (	RpairAddress#
favorite_type (RfavoriteType"+
FavoriteTokenResponse
flag (Rflag"d
GetTokenListByAddressesRequest
chain_id (RchainId'
token_addresses (	RtokenAddresses"Z
GetTokenListByAddressesResponse7
list (2#.market.GetTokenListByAddressesItemRlist"™
GetTokenListByAddressesItem
chain_id (RchainId
address (	Raddress
symbol (	Rsymbol

token_icon (	R	tokenIcon
create_time (R
createTime"d
#GetFluctuationsByPairAddressRequest!
pair_address (	RpairAddress
interval (	Rinterval"≠
$GetFluctuationsByPairAddressResponse!
pair_address (	RpairAddress0
kline_fluctuation_1m (RklineFluctuation1m0
kline_fluctuation_5m (RklineFluctuation5m2
kline_fluctuation_15m (RklineFluctuation15m0
kline_fluctuation_1h (RklineFluctuation1h0
kline_fluctuation_4h (RklineFluctuation4h2
kline_fluctuation_12h (RklineFluctuation12h2
kline_fluctuation_24h (RklineFluctuation24h"X
GetNativeTokenPriceRequest
chain_id (RchainId
search_time (	R
searchTime"N
GetNativeTokenPriceResponse/
base_token_price_usd (RbaseTokenPriceUsd"Y
GetPumpTokenInfoRequest
chain_id (RchainId#
token_address (	RtokenAddress"\
GetPairInfoByTokensRequest
chain_id (RchainId#
token_address (	RtokenAddress".
GetGasInfoRequest
chain_id (RchainId"C
GetGasInfoResponse-
gas_info (2.market.GetGasInfoRgasInfo"“

GetGasInfo
chain_id (RchainId
normal (	Rnormal

normal_usd (	R	normalUsd
fast (	Rfast
fast_usd (	RfastUsd

super_fast (	R	superFast$
super_fast_usd (	RsuperFastUsd"´
PushTokenInfoRequest
chain_id (RchainId#
token_address (	RtokenAddress!
pair_address (	RpairAddress
token_price (R
tokenPrice
mkt_cap (RmktCap

token_name (	R	tokenName!
token_symbol (	RtokenSymbol

token_icon (	R	tokenIcon
launch_time	 (R
launchTime

hold_count
 (R	holdCount
	change_24 (Rchange24
txs_24h (Rtxs24h
pump_status (R
pumpStatus"‡
PushTokenInfoResponse
chain_id (RchainId#
token_address (	RtokenAddress
txs_24h (Rtxs24h
vol_24h (	Rvol24h
	change_24 (	Rchange24
token_price (	R
tokenPrice
mkt_cap (	RmktCap"ç
'GetTokenMarketCapListByAddressesRequest
chain_id (RchainIdG

token_list (2(.market.GetTokenMarketCapListRequestItemR	tokenList"h
 GetTokenMarketCapListRequestItem#
token_address (	RtokenAddress
create_time (R
createTime"l
(GetTokenMarketCapListByAddressesResponse@
list (2,.market.GetTokenMarketCapListByAddressesItemRlist"„
$GetTokenMarketCapListByAddressesItem
chain_id (RchainId#
token_address (	RtokenAddress
mkt_cap (RmktCap
ath_mkt_cap (R	athMktCap!
pair_address (	RpairAddress
token_price (R
tokenPrice"J
GetTokenBasicListResponse-
list (2.market.GetTokenBasicItemRlist"ç
GetTokenBasicItem
chain_id (RchainId#
token_address (	RtokenAddress!
pair_address (	RpairAddress

token_name (	R	tokenName!
token_symbol (	RtokenSymbol

token_icon (	R	tokenIcon
token_price (R
tokenPrice
change24 (Rchange24!
total_supply	 (RtotalSupply
decimals
 (Rdecimals&
token_price_str (	RtokenPriceStr
fdv (Rfdv"5
GetChainTokenInfoRequest
chain_id (RchainId"„
GetChainTokenInfoResponse
chain_id (RchainId#
token_address (	RtokenAddress!
pair_address (	RpairAddress
token_price (R
tokenPrice
change24 (Rchange24&
token_price_str (	RtokenPriceStr"P
GetPairsByTimeRequest
chain_id (RchainId
	timestamp (R	timestamp">
GetPairsByTimeResponse$
list (2.market.PairInfoRlist"ê
GetKlineByPairsRequest
chain_id (RchainId

start_time (R	startTime
end_time (RendTime!
pair_address (	RpairAddress"<
GetKlineByPairsResponse!
list (2.market.KlineRlist">
TokenSecurityCheckRequest!
mint_address (	RmintAddress"˚
TokenSecurityCheckResponse!
mint_address (	RmintAddress
supply (Rsupply
decimals (Rdecimals%
is_initialized (RisInitialized%
mint_authority (	RmintAuthority)
freeze_authority (	RfreezeAuthority.
mint_authority_safe (RmintAuthoritySafe2
freeze_authority_safe (RfreezeAuthoritySafe)
security_summary	 (	RsecuritySummary"Q
GetTop10Request
chain_id (RchainId#
token_address (	RtokenAddress"(
GetTop10Response
top10 (Rtop10"T
GetContractOrTokenInfoRequest
keyword (	Rkeyword
chain_id (RchainId"h
GetContractOrToeknInfoResponse#
token (2.market.TokenRtoken!
list (2.market.TokenRlist"2
SearchTrendingRequest
chain_id (RchainId";
SearchTrendingResponse!
list (2.market.TokenRlist"Ê
Token
id (Rid
chain_id (RchainId

chain_name (	R	chainName

chain_icon (	R	chainIcon#
token_address (	RtokenAddress

token_icon (	R	tokenIcon

token_name (	R	tokenName
liq (	Rliq
mcap	 (	Rmcap
pond
 (	Rpond
price (	Rprice
	change_5m (Rchange5m
	change_1h (Rchange1h
	change_4h (Rchange4h

change_24h (R	change24h6
safety_check (2.market.SafetyCheckRsafetyCheck
is_followed (R
isFollowed!
pair_address (	RpairAddress

top_holder (R	topHolder%
lock_liquidity (RlockLiquidity
	freezable (R	freezable
mintable (Rmintable
txs_24h (Rtxs24h
vol_24h (Rvol24h

hold_count (R	holdCount"¢
SafetyCheck/
is_mintable (2.market.StatusR
isMintable1
is_freezable (2.market.StatusRisFreezable/
is_lockable (2.market.StatusR
isLockable"R
AuditInfoRequest#
token_address (	RtokenAddress
chain_id (RchainId":
AuditInfoResponse%
list (2.market.AuditInfoRlist"}
	AuditInfo
alarm (	Ralarm
quick_intel (	R
quickIntel
go_plus (	RgoPlus 
pass (2.market.PassRpass"@
Pass
quick_intel (	R
quickIntel
go_plus (	RgoPlus**
Status
UNKNOWN 	
FALSE
TRUE2•
MarketU
GetPumpTokenList.market.GetPumpTokenListRequest .market.GetPumpTokenListResponseR
GetClmmPoolList.market.GetClmmPoolListRequest.market.GetClmmPoolListResponse=
GetKline.market.GetKlineRequest.market.GetKlineResponse[
GetPairInfoByToken!.market.GetPairInfoByTokenRequest".market.GetPairInfoByTokenResponse^
GetNativeTokenPrice".market.GetNativeTokenPriceRequest#.market.GetNativeTokenPriceResponseL
PushTokenInfo.market.PushTokenInfoRequest.market.PushTokenInfoResponseI
GetTokenInfo.market.GetTokenInfoRequest.market.GetTokenInfoResponse[
TokenSecurityCheck!.market.TokenSecurityCheckRequest".market.TokenSecurityCheckResponseB
Z./marketbproto3