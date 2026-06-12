package logic

import (
	"context"
	"fmt"
	"time"

	"internalwallet/common/errcode"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/repository"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type QuerySignatureLogsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewQuerySignatureLogsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QuerySignatureLogsLogic {
	return &QuerySignatureLogsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *QuerySignatureLogsLogic) QuerySignatureLogs(in *pb.QuerySignatureLogsRequest) (*pb.QuerySignatureLogsResponse, error) {
	// 设置默认分页参数
	page := in.Page
	if page <= 0 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// 构建查询过滤器
	filter := &repository.SignatureLogFilter{
		Chain:         in.Chain,
		OperationType: int(in.OperationType),
		Status:        int(in.Status),
		Page:          int(page),
		PageSize:      int(pageSize),
	}

	// 如果有seed_id，查询获取master_seed_id
	if in.SeedId != "" {
		masterSeed, err := l.svcCtx.MasterSeedRepo.FindBySeedID(l.ctx, in.SeedId)
		if err != nil {
			l.Logger.Errorf("Failed to find seed: %v", err)
			return &pb.QuerySignatureLogsResponse{
				Code:    int32(errcode.SignerSeedNotFound),
				Message: "seed not found",
			}, nil
		}
		filter.SeedID = &masterSeed.ID
	}

	// 解析日期
	if in.StartDate != "" {
		startTime, err := time.Parse("2006-01-02", in.StartDate)
		if err == nil {
			filter.StartDate = &startTime
		}
	}
	if in.EndDate != "" {
		endTime, err := time.Parse("2006-01-02", in.EndDate)
		if err == nil {
			// 包含整天
			endTime = endTime.Add(24 * time.Hour)
			filter.EndDate = &endTime
		}
	}

	// 调用Repository查询
	logs, total, err := l.svcCtx.SignatureLogRepo.List(l.ctx, filter)
	if err != nil {
		l.Logger.Errorf("Failed to query signature logs: %v", err)
		return &pb.QuerySignatureLogsResponse{
			Code:    int32(errcode.SignerDatabaseError),
			Message: "failed to query logs",
		}, nil
	}

	// 转换为响应格式
	records := make([]*pb.SignatureLogInfo, 0, len(logs))
	for _, log := range logs {
		record := &pb.SignatureLogInfo{
			RequestId:      log.RequestID,
			SeedId:         fmt.Sprintf("%d", log.MasterSeedID), // TODO: 优化为JOIN查询获取实际seed_id
			Chain:          log.Chain,
			DerivationPath: log.DerivationPath,
			OperationType:  int32(log.OperationType),
			Status:         int32(log.Status),
			CreatedAt:      log.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if log.Amount != nil {
			record.Amount = *log.Amount
		}
		records = append(records, record)
	}

	return &pb.QuerySignatureLogsResponse{
		Code:     0,
		Message:  "success",
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Records:  records,
	}, nil
}
