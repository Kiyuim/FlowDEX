package logic

import (
	"context"
	"fmt"
	"time"

	"dex/pkg/constants"
	trade2 "dex/pkg/trade"
	"dex/trade/internal/svc"
	"dex/trade/trade"

	aSDK "github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePoolLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreatePoolLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePoolLogic {
	return &CreatePoolLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreatePoolLogic) CreatePool(in *trade.CreatePoolRequest) (*trade.CreatePoolResponse, error) {
	l.Infof("🏊 CreatePool request: ChainId=%d, TokenMint0=%s, TokenMint1=%s, Price=%s, FeeTier=%d, OpenTime=%d",
		in.ChainId, in.TokenMint_0, in.TokenMint_1, in.InitialPrice, in.FeeTier, in.OpenTime)

	// Validate parameters
	if err := l.validateCreatePoolParams(in); err != nil {
		l.Errorf("❌ Parameter validation failed: %v", err)
		return nil, err
	}

	// Only support Solana for now
	if in.ChainId != constants.SolChainIdInt {
		return nil, fmt.Errorf("unsupported chain ID: %d, only Solana (%d) is supported", in.ChainId, constants.SolChainIdInt)
	}

	// Check if TxManager is available
	if l.svcCtx.SolTxMananger == nil {
		return nil, fmt.Errorf("SolTxMananger is nil - check if Sol configuration is enabled")
	}

	// Convert to internal format for pool creation
	createPoolTx := &trade2.CreatePoolTx{
		ChainId:           int64(in.ChainId),
		TokenMint0:        in.TokenMint_0,
		TokenMint1:        in.TokenMint_1,
		InitialPrice:      in.InitialPrice,
		FeeTier:           in.FeeTier,
		OpenTime:          in.OpenTime,
		UserWalletAddress: in.UserWalletAddress,
	}

	// Build unsigned transaction for third-party wallet signing
	unsignedTxBase64, poolAddress, err := l.svcCtx.SolTxMananger.BuildUnsignedPoolTransaction(l.ctx, createPoolTx)
	if err != nil {
		l.Errorf("❌ SolTxMananger.BuildUnsignedPoolTransaction err: %v", err)
		return nil, err
	}

	l.Infof("✅ BuildUnsignedPoolTransaction success, length=%d, poolAddress=%s", len(unsignedTxBase64), poolAddress)
	return &trade.CreatePoolResponse{TxHash: unsignedTxBase64, PoolAddress: poolAddress}, nil
}

func (l *CreatePoolLogic) validateCreatePoolParams(in *trade.CreatePoolRequest) error {
	// Validate wallet address
	if len(in.UserWalletAddress) == 0 {
		return fmt.Errorf("user wallet address is required")
	}

	_, err := aSDK.PublicKeyFromBase58(in.UserWalletAddress)
	if err != nil {
		return fmt.Errorf("invalid user wallet address: %v", err)
	}

	// Validate token mints
	if len(in.TokenMint_0) == 0 || len(in.TokenMint_1) == 0 {
		return fmt.Errorf("both token mints are required")
	}

	_, err = aSDK.PublicKeyFromBase58(in.TokenMint_0)
	if err != nil {
		return fmt.Errorf("invalid token_mint_0: %v", err)
	}

	_, err = aSDK.PublicKeyFromBase58(in.TokenMint_1)
	if err != nil {
		return fmt.Errorf("invalid token_mint_1: %v", err)
	}

	// Ensure tokens are different
	if in.TokenMint_0 == in.TokenMint_1 {
		return fmt.Errorf("token mints must be different")
	}

	// Validate initial price
	if len(in.InitialPrice) == 0 {
		return fmt.Errorf("initial price is required")
	}

	priceDecimal, err := decimal.NewFromString(in.InitialPrice)
	if err != nil {
		return fmt.Errorf("invalid initial price format: %v", err)
	}

	if !priceDecimal.IsPositive() {
		return fmt.Errorf("initial price must be positive")
	}

	// Validate fee tier (in basis points)
	validFeeTiers := []int32{1, 5, 30, 100, 500, 1000} // 0.01%, 0.05%, 0.3%, 1%, 5%, 10%
	isValidFeeTier := false
	for _, validTier := range validFeeTiers {
		if in.FeeTier == validTier {
			isValidFeeTier = true
			break
		}
	}
	if !isValidFeeTier {
		return fmt.Errorf("invalid fee tier: %d. Valid tiers are: %v (in basis points)", in.FeeTier, validFeeTiers)
	}

	// Set open time to current time if not specified
	if in.OpenTime <= 0 {
		in.OpenTime = time.Now().Unix() // Default to current time
	}

	// Removed check that prevented open_time from being in the past
	// Always use the time provided by the client, or current time if not specified

	l.Infof("✅ Pool parameters validated successfully")
	return nil
}
