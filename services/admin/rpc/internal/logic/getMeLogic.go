package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetMeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMeLogic {
	return &GetMeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetMeLogic) GetMe(in *pb.GetMeRequest) (*pb.GetMeResponse, error) {
	_ = in

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	rbacResp, err := NewGetMyRBACLogic(l.ctx, l.svcCtx).GetMyRBAC(&pb.GetMyRBACRequest{})
	if err != nil {
		return nil, err
	}

	twoFARequired := false
	if l.svcCtx != nil {
		twoFARequired = l.svcCtx.EffectiveRequire2FA(l.ctx) || current.TwoFactorRequired
	} else {
		twoFARequired = current.TwoFactorRequired
	}

	userInfo := &pb.AdminUserInfo{
		AdminId:               resp.AdminIDString(current.ID),
		Username:              current.Username,
		Name:                  current.Name,
		Role:                  current.Role,
		Permissions:           nil,
		RequirePasswordChange: current.RequirePasswordChange,
	}
	if rbacResp != nil && rbacResp.Data != nil {
		if rbacResp.Data.Role != "" {
			userInfo.Role = rbacResp.Data.Role
		}
		userInfo.Permissions = rbacResp.Data.UserPermissionCodes
	}

	// Hot wallet runtime state (best-effort)
	walletState := ""
	if l.svcCtx != nil && l.svcCtx.SignerRpc != nil {
		if st, err := l.svcCtx.SignerRpc.GetRuntimeStatus(l.ctx, &pb.GetRuntimeStatusRequest{}); err == nil && st != nil {
			walletState = st.GetWalletState()
		}
	}

	return &pb.GetMeResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetMeData{
			UserInfo:          userInfo,
			Status:            current.Status,
			TwoFactorEnabled:  current.TwoFactorEnabled,
			TwoFactorRequired: twoFARequired,
			LastLoginAt:       formatTimePtr(current.LastLoginAt),
			LastLoginIp:       current.LastLoginIP,
			CreatedAt:         formatTime(current.CreatedAt),
			UpdatedAt:         formatTime(current.UpdatedAt),
			Rbac:              rbacResp.GetData(),
			WalletState:       walletState,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
