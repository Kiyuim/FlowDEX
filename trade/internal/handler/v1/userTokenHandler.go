//go:build ignore

// This file implements a REST layer (UserTokens/UserPools) that is never wired
// into trade.go's main() (trade only starts a gRPC server — see trade/trade.go).
// It also doesn't compile as-is (undefined xcode.ParamsError, missing
// svcCtx.UserPoolsModel/UserTokensModel, wrong xcode.New signature). Excluded
// from the build rather than silently left broken; see
// docs/项目已知问题与修复记录.md for what's needed to finish it.

package v1

import (
	"dex/pkg/xcode"
	"dex/trade/internal/svc"
	"dex/trade/internal/types"
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// StoreUserTokenHandler handles storing a user-created token
func StoreUserTokenHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.StoreUserTokenReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, xcode.ParamsError.Wrap(err))
			return
		}

		l := v1.NewStoreUserTokenLogic(r.Context(), svcCtx)
		resp, err := l.StoreUserToken(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

// GetUserTokensHandler handles retrieving user-created tokens
func GetUserTokensHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetUserTokensReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, xcode.ParamsError.Wrap(err))
			return
		}

		l := v1.NewGetUserTokensLogic(r.Context(), svcCtx)
		resp, err := l.GetUserTokens(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

// StoreUserPoolHandler handles storing a user-created pool
func StoreUserPoolHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.StoreUserPoolReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, xcode.ParamsError.Wrap(err))
			return
		}

		l := v1.NewStoreUserPoolLogic(r.Context(), svcCtx)
		resp, err := l.StoreUserPool(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

// GetUserPoolsHandler handles retrieving user-created pools
func GetUserPoolsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetUserPoolsReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, xcode.ParamsError.Wrap(err))
			return
		}

		l := v1.NewGetUserPoolsLogic(r.Context(), svcCtx)
		resp, err := l.GetUserPools(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
