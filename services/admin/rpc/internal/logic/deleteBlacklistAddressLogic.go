package logic

import (
	"context"
	"encoding/json"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type DeleteBlacklistAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteBlacklistAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteBlacklistAddressLogic {
	return &DeleteBlacklistAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteBlacklistAddressLogic) DeleteBlacklistAddress(in *pb.DeleteBlacklistAddressRequest) (*pb.DeleteBlacklistAddressResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "id required", map[string]string{"id": "required"})
	}
	if l.svcCtx == nil || l.svcCtx.DB == nil || l.svcCtx.BlacklistAddressRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		repo := l.svcCtx.BlacklistAddressRepo.WithTx(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		m, err := repo.FindByID(l.ctx, in.Id)
		if err != nil || m == nil {
			return errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "blacklist address not found", nil)
		}

		if err := repo.SoftDelete(l.ctx, in.Id, now); err != nil {
			return err
		}

		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"id":         in.Id,
			"address":    maskAddress(m.Address),
			"network":    m.Network,
			"risk_level": m.RiskLevel,
			"source":     m.Source,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "blacklist.address.delete",
			TargetType:  "blacklist_address",
			TargetID:    maskAddress(m.Address),
			Description: "删除黑名单地址: " + maskAddress(m.Address),
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		l.Logger.Errorf("delete blacklist address failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "delete failed", nil)
	}

	return &pb.DeleteBlacklistAddressResponse{
		Success:   true,
		Message:   "ok",
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
