package logic

import (
	"context"
	"net/url"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type UpdateChainIconLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateChainIconLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateChainIconLogic {
	return &UpdateChainIconLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateChainIconLogic) UpdateChainIcon(in *pb.UpdateChainIconRequest) (*pb.UpdateChainIconResponse, error) {
	if in == nil || strings.TrimSpace(in.ChainCode) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{"chain_code": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.ChainRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	if _, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	chainCode := normalizeCode(in.ChainCode)
	iconURL := strings.TrimSpace(in.IconUrl)
	if iconURL != "" {
		if len(iconURL) > 2048 {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ICON_URL", "icon_url too long", map[string]string{"icon_url": "too long"})
		}
		u, parseErr := url.ParseRequestURI(iconURL)
		if parseErr != nil || u == nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ICON_URL", "invalid icon_url", map[string]string{"icon_url": "invalid"})
		}
	}

	if err := l.svcCtx.ChainRepo.UpdateIconURLByCode(l.ctx, chainCode, iconURL); err != nil {
		if table, ok := errx.MySQLTableNotFound(err); ok {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING", "missing table: "+table, nil)
		}
		if col, ok := errx.MySQLColumnNotFound(err); ok {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING", "missing column: "+col, nil)
		}
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "chain not found", map[string]string{"chain_code": "not found"})
		}
		l.Logger.Errorf("update chain icon_url failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	return &pb.UpdateChainIconResponse{
		Success: true,
		Message: "ok",
		Data: &pb.ChainItem{
			ChainCode: chainCode,
			IconUrl:   iconURL,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
