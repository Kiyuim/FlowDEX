package logic

import (
	"context"
	"encoding/json"
	"time"

	"dex/market/internal/svc"
	"dex/market/market"
	"dex/model/solmodel"

	"github.com/zeromicro/go-zero/core/logx"
)

type PushTokenInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPushTokenInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PushTokenInfoLogic {
	return &PushTokenInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Push token information to websocket
func (l *PushTokenInfoLogic) PushTokenInfo(in *market.PushTokenInfoRequest) (*market.PushTokenInfoResponse, error) {
	// Use the token metadata passed from consumer, with database enhancement
	tokenName := in.TokenName
	tokenSymbol := in.TokenSymbol
	tokenIcon := in.TokenIcon
	launchTime := in.LaunchTime
	var twitterUsername, telegram string
	var holdCount int64

	// If no launch time provided, use current time
	if launchTime == 0 {
		launchTime = time.Now().Unix()
	}

	// Get complete token info from database (following getpumptokenlist logic)
	tokenModel := solmodel.NewTokenModel(l.svcCtx.DB)
	tokenInfo, err := tokenModel.FindOneByChainIdAddress(l.ctx, in.ChainId, in.TokenAddress)
	if err == nil && tokenInfo != nil {
		// Use database values if not provided
		if tokenName == "" {
			tokenName = tokenInfo.Name
		}
		if tokenIcon == "" {
			tokenIcon = tokenInfo.Icon
		}
		// Get social media info
		twitterUsername = tokenInfo.TwitterUsername
		telegram = tokenInfo.Telegram

		// Calculate holder count (following getpumptokenlist logic)
		solTokenAccountModel := solmodel.NewSolTokenAccountModel(l.svcCtx.DB)
		holders, err := solTokenAccountModel.CountByTokenAddressWithTime(l.ctx, in.ChainId, in.TokenAddress, tokenInfo.CreatedAt)
		if err != nil {
			l.Errorf("Failed to count token holders: %v", err)
			holdCount = 0
		} else {
			holdCount = holders
		}
	} else {
		l.Infof("Token info not found in database for %s: %v", in.TokenAddress, err)
		holdCount = 0
	}

	// Prepare complete token data for WebSocket broadcast (enhanced with database info)
	tokenData := map[string]interface{}{
		"chain_id":         in.ChainId,
		"token_address":    in.TokenAddress,
		"pair_address":     in.PairAddress,
		"token_price":      in.TokenPrice,
		"mkt_cap":          in.MktCap,
		"token_name":       tokenName,
		"token_symbol":     tokenSymbol,
		"token_icon":       tokenIcon,
		"launch_time":      launchTime,
		"hold_count":       holdCount, // Real holder count from database
		"change_24":        in.Change_24,
		"txs_24h":          in.Txs_24H,
		"pump_status":      in.PumpStatus,
		"twitter_username": twitterUsername, // Social media info
		"telegram":         telegram,        // Social media info
	}

	// Convert to JSON
	jsonData, err := json.Marshal(tokenData)
	if err != nil {
		l.Errorf("Failed to marshal token data: %v", err)
		return &market.PushTokenInfoResponse{
			ChainId:      in.ChainId,
			TokenAddress: in.TokenAddress,
			Txs_24H:      0,
			Vol_24H:      "0",
			Change_24:    "0",
			TokenPrice:   "0",
			MktCap:       "0",
		}, nil
	}

	// Publish to Redis for new token creation
	channel := "pump_token_new"
	_, err = l.svcCtx.RDS.Publish(channel, string(jsonData))
	if err != nil {
		l.Errorf("Failed to publish token info to Redis: %v", err)
		return &market.PushTokenInfoResponse{
			ChainId:      in.ChainId,
			TokenAddress: in.TokenAddress,
			Txs_24H:      0,
			Vol_24H:      "0",
			Change_24:    "0",
			TokenPrice:   "0",
			MktCap:       "0",
		}, nil
	}

	l.Infof("Successfully published token info to channel %s", channel)

	return &market.PushTokenInfoResponse{
		ChainId:      in.ChainId,
		TokenAddress: in.TokenAddress,
		Txs_24H:      0,
		Vol_24H:      "0",
		Change_24:    "0",
		TokenPrice:   "0",
		MktCap:       "0",
	}, nil
}
