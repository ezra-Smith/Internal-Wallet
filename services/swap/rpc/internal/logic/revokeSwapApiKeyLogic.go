package logic

import (
	"context"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RevokeSwapApiKeyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRevokeSwapApiKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RevokeSwapApiKeyLogic {
	return &RevokeSwapApiKeyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RevokeSwapApiKeyLogic) RevokeSwapApiKey(in *pb.RevokeSwapApiKeyRequest) (*pb.RevokeSwapApiKeyResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if l.svcCtx.DB == nil || l.svcCtx.ApiKeyRepo == nil {
		return nil, status.Error(codes.Internal, "db not configured")
	}

	if err := l.svcCtx.ApiKeyRepo.UpdateFields(l.ctx, in.Id, map[string]any{
		"is_active":  0,
		"updated_at": time.Now(),
	}); err != nil {
		l.Logger.Errorw("revoke swap api key failed", logx.Field("error", err), logx.Field("id", in.Id))
		return nil, status.Error(codes.Internal, "failed to revoke api key")
	}

	return &pb.RevokeSwapApiKeyResponse{
		Success: true,
		Message: "ok",
	}, nil
}
