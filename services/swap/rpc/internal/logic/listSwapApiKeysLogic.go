package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListSwapApiKeysLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSwapApiKeysLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSwapApiKeysLogic {
	return &ListSwapApiKeysLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListSwapApiKeysLogic) ListSwapApiKeys(in *pb.ListSwapApiKeysRequest) (*pb.ListSwapApiKeysResponse, error) {
	if l.svcCtx.DB == nil || l.svcCtx.ApiKeyRepo == nil {
		return nil, status.Error(codes.Internal, "db not configured")
	}

	page := int32(1)
	pageSize := int32(20)
	if in != nil {
		if in.Page > 0 {
			page = in.Page
		}
		if in.PageSize > 0 {
			pageSize = in.PageSize
		}
	}

	items, total, err := l.svcCtx.ApiKeyRepo.List(l.ctx, page, pageSize)
	if err != nil {
		l.Logger.Errorw("list swap api keys failed", logx.Field("error", err))
		return nil, status.Error(codes.Internal, "failed to list api keys")
	}

	out := make([]*pb.SwapApiKeyItem, 0, len(items))
	for _, it := range items {
		if it == nil {
			continue
		}
		var expiresAtUnix int64
		if it.ExpiresAt != nil {
			expiresAtUnix = it.ExpiresAt.Unix()
		}
		out = append(out, &pb.SwapApiKeyItem{
			Id:            it.ID,
			ProjectName:   it.ProjectName,
			KeyName:       it.KeyName,
			RateLimitTier: it.RateLimitTier,
			IsActive:      it.IsActive,
			TokenDisplay:  it.TokenDisplay,
			CreatedAtUnix: it.CreatedAt.Unix(),
			ExpiresAtUnix: expiresAtUnix,
		})
	}

	pagination := buildPagination(page, pageSize, total)

	return &pb.ListSwapApiKeysResponse{
		Success:    true,
		Message:    "ok",
		Items:      out,
		Pagination: pagination,
	}, nil
}

func buildPagination(page, pageSize int32, total int64) *pb.SwapPagination {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	var totalPages int32
	if total > 0 {
		totalPages = int32((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return &pb.SwapPagination{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
		HasPrev:    page > 1,
		HasNext:    totalPages > 0 && page < totalPages,
	}
}
