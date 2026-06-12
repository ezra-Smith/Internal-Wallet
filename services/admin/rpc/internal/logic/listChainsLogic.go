package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type ListChainsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListChainsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListChainsLogic {
	return &ListChainsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// -------- Chains --------
func (l *ListChainsLogic) ListChains(in *pb.ListChainsRequest) (*pb.ListChainsResponse, error) {
	if in == nil {
		in = &pb.ListChainsRequest{}
	}
	if l.svcCtx.DB == nil || l.svcCtx.ChainRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	var rowsErr error
	var rows []*model.ChainModel
	if in.IncludeDisabled {
		rows, rowsErr = l.svcCtx.ChainRepo.ListAll(l.ctx)
	} else {
		rows, rowsErr = l.svcCtx.ChainRepo.ListEnabled(l.ctx)
	}
	if rowsErr != nil {
		if table, ok := errx.MySQLTableNotFound(rowsErr); ok {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING", "missing table: "+table, nil)
		}
		l.Logger.Errorf("list chains failed: %v", rowsErr)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	items := make([]*pb.ChainItem, 0, len(rows))
	for _, it := range rows {
		if it == nil {
			continue
		}
		code := strings.ToUpper(strings.TrimSpace(it.Name))
		items = append(items, &pb.ChainItem{
			ChainCode:   code,
			Network:     strings.TrimSpace(it.Network),
			ChainId:     it.ChainID,
			ExplorerUrl: strings.TrimSpace(it.ExplorerUrl),
			Status:      it.Status,
			IconUrl:     strings.TrimSpace(it.IconUrl),
		})
	}

	return &pb.ListChainsResponse{
		Success: true,
		Message: "ok",
		Data: &pb.ListChainsData{
			Chains: items,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
