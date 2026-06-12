package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/pkg/accounting"
	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/repository"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListUserTransactionRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListUserTransactionRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUserTransactionRecordsLogic {
	return &ListUserTransactionRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListUserTransactionRecordsLogic) ListUserTransactionRecords(in *pb.ListUserTransactionRecordsRequest) (*pb.ListUserTransactionRecordsResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.ListUserTransactionRecordsResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.UserTxRepo == nil {
		return &pb.ListUserTransactionRecordsResponse{Success: false, Message: "repository not initialized"}, nil
	}
	if in == nil {
		in = &pb.ListUserTransactionRecordsRequest{}
	}
	// user_id 是可选参数，0 表示查询所有用户

	var startTime *time.Time
	if in.StartTime > 0 {
		t := time.Unix(in.StartTime, 0).Local()
		startTime = &t
	}
	var endTime *time.Time
	if in.EndTime > 0 {
		t := time.Unix(in.EndTime, 0).Local()
		endTime = &t
	}

	f := repository.UserTransactionRecordListFilter{
		UserID:    in.UserId,
		TxType:    in.TxType,
		AssetCode: accounting.NormalizeAssetCode(in.AssetCode),
		ChainCode: accounting.NormalizeChainCode(in.ChainCode),
		Status:    strings.ToLower(strings.TrimSpace(in.Status)),
		StartTime: startTime,
		EndTime:   endTime,
	}

	rows, total, err := l.svcCtx.UserTxRepo.List(l.ctx, in.Page, in.PageSize, f)
	if err != nil {
		l.Logger.Errorf("ListUserTransactionRecords failed: %v", err)
		return &pb.ListUserTransactionRecordsResponse{Success: false, Message: err.Error()}, nil
	}

	items := make([]*pb.UserTransactionRecordItem, 0, len(rows))
	for _, r := range rows {
		rr := r
		items = append(items, toPBUserTxRecordItem(&rr))
	}
	return &pb.ListUserTransactionRecordsResponse{
		Success: true,
		Message: "ok",
		Total:   total,
		Items:   items,
	}, nil
}
