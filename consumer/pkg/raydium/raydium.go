package raydium

import (
	"context"
	"fmt"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/blocto/solana-go-sdk/rpc"
	"github.com/gagliardetto/solana-go"
	"github.com/near/borsh-go"
)

const (
	Pump2RaydiumInitTokenAmount     = 206900000000000
	Pump2RaydiumInitBaseTokenAmount = 79005359270
)

type Fees struct {
	MinSeparateNumerator   uint64 `json:"min_separate_numerator"`
	MinSeparateDenominator uint64 `json:"min_separate_denominator"`

	TradeFeeNumerator   uint64 `json:"trade_fee_numerator"`
	TradeFeeDenominator uint64 `json:"trade_fee_denominator"`

	PnlNumerator   uint64 `json:"pnl_numerator"`
	PnlDenominator uint64 `json:"pnl_denominator"`

	SwapFeeNumerator   uint64 `json:"swap_fee_numerator"`
	SwapFeeDenominator uint64 `json:"swap_fee_denominator"`
}

type StateData struct {
	NeedTakePnlCoin     uint64    `json:"need_take_pnl_coin"`
	NeedTakePnlPc       uint64    `json:"need_take_pnl_pc"`
	TotalPnlPc          uint64    `json:"total_pnl_pc"`
	TotalPnlCoin        uint64    `json:"total_pnl_coin"`
	PoolOpenTime        uint64    `json:"pool_open_time"`
	Padding             [2]uint64 `json:"padding"`
	OrderbookToInitTime uint64    `json:"orderbook_to_init_time"`
	SwapCoinInAmount    [2]uint64 `json:"swap_coin_in_amount"`
	SwapPcOutAmount     [2]uint64 `json:"swap_pc_out_amount"`
	SwapAccPcFee        uint64    `json:"swap_acc_pc_fee"`
	SwapPcInAmount      [2]uint64 `json:"swap_pc_in_amount"`
	SwapCoinOutAmount   [2]uint64 `json:"swap_coin_out_amount"`
	SwapAccCoinFee      uint64    `json:"swap_acc_coin_fee"`
}

type AmmInfo struct {
	Status             uint64           `json:"status"`
	Nonce              uint64           `json:"nonce"`
	OrderNum           uint64           `json:"order_num"`
	Depth              uint64           `json:"depth"`
	CoinDecimals       uint64           `json:"coin_decimals"`
	PcDecimals         uint64           `json:"pc_decimals"`
	State              uint64           `json:"state"`
	ResetFlag          uint64           `json:"reset_flag"`
	MinSize            uint64           `json:"min_size"`
	VolMaxCutRatio     uint64           `json:"vol_max_cut_ratio"`
	AmountWave         uint64           `json:"amount_wave"`
	CoinLotSize        uint64           `json:"coin_lot_size"`
	PcLotSize          uint64           `json:"pc_lot_size"`
	MinPriceMultiplier uint64           `json:"min_price_multiplier"`
	MaxPriceMultiplier uint64           `json:"max_price_multiplier"`
	SysDecimalValue    uint64           `json:"sys_decimal_value"`
	Fees               Fees             `json:"fees"`
	StateData          StateData        `json:"state_data"`
	CoinVault          solana.PublicKey `json:"coin_vault"`
	PcVault            solana.PublicKey `json:"pc_vault"`
	CoinVaultMint      solana.PublicKey `json:"coin_vault_mint"`
	PcVaultMint        solana.PublicKey `json:"pc_vault_mint"`
	LpMint             solana.PublicKey `json:"lp_mint"`
	OpenOrders         solana.PublicKey `json:"open_orders"`
	Market             solana.PublicKey `json:"market"`
	MarketProgram      solana.PublicKey `json:"market_program"`
	TargetOrders       solana.PublicKey `json:"target_orders"`
	Padding1           [8]uint64        `json:"padding1"`
	AmmOwner           solana.PublicKey `json:"amm_owner"`
	LpAmount           uint64           `json:"lp_amount"`
	ClientOrderID      uint64           `json:"client_order_id"`
	RecentEpoch        uint64           `json:"recent_epoch"`
	Padding2           uint64           `json:"padding2"`
}

func GetAmmPoolInfo(c *client.Client, ctx context.Context, address string) (*AmmInfo, error) {
	resp, err := c.GetAccountInfoWithConfig(ctx, address, client.GetAccountInfoConfig{
		Commitment: rpc.CommitmentConfirmed,
	})
	if err != nil {
		err = fmt.Errorf("GetAmmPoolInfo:GetAccountInfoWithConfig err:%v,token address: %v", err, address)
		return nil, err
	}

	var ammInfo AmmInfo
	err = borsh.Deserialize(&ammInfo, resp.Data)
	if err != nil {
		err = fmt.Errorf("GetAmmPoolInfo:Deserialize err:%v,token address: %v", err, address)
		return nil, err
	}

	return &ammInfo, nil
}
