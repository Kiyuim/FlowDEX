package logic

import (
	"context"
	"fmt"

	"dex/pkg/constants"
	tradepkg "dex/pkg/trade"
	"dex/trade/internal/svc"
	"dex/trade/trade"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

type AddLiquidityV1Logic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddLiquidityV1Logic(ctx context.Context, svcCtx *svc.ServiceContext) *AddLiquidityV1Logic {
	return &AddLiquidityV1Logic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AddLiquidityV1Logic) AddLiquidityV1(in *trade.AddLiquidityRequest) (*trade.AddLiquidityResponse, error) {
	// Log the incoming request with detailed information
	l.Infof("🔍 AddLiquidity RPC called with request: %+v", in)
	l.Infof("🔍 Request details - ChainId: %d, PoolId: %s, BaseToken: %d, TokenA: %s, TokenB: %s",
		in.ChainId, in.PoolId, in.BaseToken, in.TokenAAddress, in.TokenBAddress)
	l.Infof("🔍 Amount details - BaseAmount: %s, OtherAmountMax: %s, TickLower: %d, TickUpper: %d",
		in.BaseAmount, in.OtherAmountMax, in.TickLower, in.TickUpper)

	// 1. Validate the request parameters
	l.Infof("🔍 Step 1: Validating request parameters")
	if err := l.validateRequest(in); err != nil {
		l.Errorf("❌ Invalid request: %v", err)
		return nil, fmt.Errorf("validation error: %v", err)
	}
	l.Infof("✅ Request validation successful")

	// 2. Check if chain is supported
	l.Infof("🔍 Step 2: Checking if chain is supported (expected: %d)", constants.SolChainIdInt)
	if in.ChainId != constants.SolChainIdInt {
		l.Errorf("❌ Unsupported chain ID: %d, only Solana (%d) is supported", in.ChainId, constants.SolChainIdInt)
		return nil, fmt.Errorf("unsupported chain ID: %d, only Solana (%d) is supported", in.ChainId, constants.SolChainIdInt)
	}
	l.Infof("✅ Chain ID check successful")

	// 3. Check if TxManager is available
	l.Infof("🔍 Step 3: Checking if SolTxManager is available")
	if l.svcCtx.SolTxMananger == nil {
		l.Errorf("❌ SolTxMananger is nil - check if Sol configuration is enabled")
		return nil, fmt.Errorf("SolTxMananger is nil - check if Sol configuration is enabled")
	}
	l.Infof("✅ SolTxManager is available")

	// 4. Convert to internal AddLiquidityTx type
	l.Infof("🔍 Step 4: Converting to internal AddLiquidityTx type")
	addLiquidityTx := &tradepkg.AddLiquidityTx{
		ChainId:           in.ChainId,
		PoolId:            in.PoolId,
		TickLower:         in.TickLower,
		TickUpper:         in.TickUpper,
		BaseToken:         in.BaseToken,
		BaseAmount:        in.BaseAmount,
		OtherAmountMax:    in.OtherAmountMax,
		UserWalletAddress: in.UserWalletAddress,
		TokenAAddress:     in.TokenAAddress,
		TokenBAddress:     in.TokenBAddress,
	}
	l.Infof("✅ Conversion successful")

	// 5. Build unsigned transaction for third-party wallet signing
	l.Infof("🔍 Step 5: Building unsigned transaction via SolTxManager.BuildUnsignedAddLiquidityTransaction")
	unsignedTxBase64, _, err := l.svcCtx.SolTxMananger.BuildUnsignedAddLiquidityTransaction(l.ctx, addLiquidityTx)
	if err != nil {
		l.Errorf("❌ SolTxMananger.BuildUnsignedAddLiquidityTransaction error: %v", err)
		return nil, fmt.Errorf("failed to build transaction: %v", err)
	}

	l.Infof("✅ BuildUnsignedAddLiquidityTransaction success, length=%d", len(unsignedTxBase64))
	return &trade.AddLiquidityResponse{
		TxHash: unsignedTxBase64,
	}, nil
}

func (l *AddLiquidityV1Logic) validateRequest(in *trade.AddLiquidityRequest) error {
	if in.PoolId == "" {
		return fmt.Errorf("pool ID cannot be empty")
	}

	if in.UserWalletAddress == "" {
		return fmt.Errorf("user wallet address cannot be empty")
	}

	if in.BaseAmount == "" {
		return fmt.Errorf("base amount cannot be empty")
	}

	if in.OtherAmountMax == "" {
		return fmt.Errorf("other amount max cannot be empty")
	}

	if in.TokenAAddress == "" || in.TokenBAddress == "" {
		return fmt.Errorf("token addresses cannot be empty")
	}

	// Validate numeric values
	_, err := decimal.NewFromString(in.BaseAmount)
	if err != nil {
		return fmt.Errorf("invalid base amount: %v", err)
	}

	_, err = decimal.NewFromString(in.OtherAmountMax)
	if err != nil {
		return fmt.Errorf("invalid other amount max: %v", err)
	}

	// Validate base token index
	if in.BaseToken != 0 && in.BaseToken != 1 {
		return fmt.Errorf("base token must be 0 (for token A) or 1 (for token B)")
	}

	return nil
}
