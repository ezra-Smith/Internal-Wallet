package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetInternalTransferLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetInternalTransferLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetInternalTransferLogic {
	return &GetInternalTransferLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetInternalTransferLogic) GetInternalTransfer(in *pb.GetInternalTransferRequest) (*pb.GetInternalTransferResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id", map[string]string{"id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.CurrencyTransferOrderRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	if _, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	order, err := l.svcCtx.CurrencyTransferOrderRepo.GetByID(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("get internal transfer failed: %v", err)
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "transfer not found", nil)
	}

	item := toPBInternalTransferItem(order)
	// Fetch user info if needed
	if l.svcCtx.UserRepo != nil {
		if fromUser, err := l.svcCtx.UserRepo.FindByID(l.ctx, order.FromUserID); err == nil && fromUser != nil {
			item.FromUserEmail = strings.TrimSpace(fromUser.Email)
			item.FromUserPhone = strings.TrimSpace(fromUser.Phone)
		}
		if toUser, err := l.svcCtx.UserRepo.FindByID(l.ctx, order.ToUserID); err == nil && toUser != nil {
			item.ToUserEmail = strings.TrimSpace(toUser.Email)
			item.ToUserPhone = strings.TrimSpace(toUser.Phone)
		}
	}

	return &pb.GetInternalTransferResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetInternalTransferData{
			Transfer: item,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
