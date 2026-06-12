package logic

import (
	"context"
	"fmt"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetUsersStatisticsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUsersStatisticsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUsersStatisticsLogic {
	return &GetUsersStatisticsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUsersStatisticsLogic) GetUsersStatistics(in *pb.GetUsersStatisticsRequest) (*pb.GetUsersStatisticsResponse, error) {
	if in == nil {
		in = &pb.GetUsersStatisticsRequest{}
	}
	if l.svcCtx.DB == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	var web2Total int64
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.UserModel{}).
		Where("deleted_at IS NULL").
		Count(&web2Total).Error; err != nil {
		if table, ok := errx.MySQLTableNotFound(err); ok {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING", "missing table: "+table, nil)
		}
		l.Logger.Errorf("count web2 users failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	var web3Total int64
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.Web3UserModel{}).
		Where("deleted_at IS NULL").
		Count(&web3Total).Error; err != nil {
		if table, ok := errx.MySQLTableNotFound(err); ok {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING", "missing table: "+table, nil)
		}
		l.Logger.Errorf("count web3 users failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	var whitelistTotal int64
	type cntRow struct {
		Cnt int64 `gorm:"column:cnt"`
	}
	var r cntRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw(`
SELECT COUNT(DISTINCT t.user_id) AS cnt
FROM (
  SELECT user_id
  FROM user_withdraw_audit_whitelist_rules
  WHERE deleted_at IS NULL AND enabled = 1
  UNION
  SELECT user_id
  FROM user_whitelist_settings
  WHERE deleted_at IS NULL AND bypass_withdraw_audit = 1
) t
`).
		Scan(&r).Error; err != nil {
		if table, ok := errx.MySQLTableNotFound(err); ok {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING", "missing table: "+table, nil)
		}
		l.Logger.Errorf("count whitelist users failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	whitelistTotal = r.Cnt

	totalUsers := web2Total + web3Total

	return &pb.GetUsersStatisticsResponse{
		Success: true,
		Message: "ok",
		Data: &pb.UsersStatisticsData{
			Web2Total:      fmt.Sprintf("%d", web2Total),
			Web3Total:      fmt.Sprintf("%d", web3Total),
			WhitelistTotal: fmt.Sprintf("%d", whitelistTotal),
			TotalUsers:     fmt.Sprintf("%d", totalUsers),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
