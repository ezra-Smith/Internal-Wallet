package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpsertUserTransactionRecordMetaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertUserTransactionRecordMetaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertUserTransactionRecordMetaLogic {
	return &UpsertUserTransactionRecordMetaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpsertUserTransactionRecordMetaLogic) UpsertUserTransactionRecordMeta(in *pb.UpsertUserTransactionRecordMetaRequest) (*pb.UpsertUserTransactionRecordMetaResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.UpsertUserTransactionRecordMetaResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.UserTxRepo == nil {
		return &pb.UpsertUserTransactionRecordMetaResponse{Success: false, Message: "repository not initialized"}, nil
	}
	if in == nil || in.Id <= 0 {
		return &pb.UpsertUserTransactionRecordMetaResponse{Success: false, Message: "invalid params"}, nil
	}

	fields := map[string]any{}
	if in.TxType > 0 {
		fields["tx_type"] = in.TxType
	}
	if v := strings.TrimSpace(in.Status); v != "" {
		fields["status"] = strings.ToLower(v)
	}
	if v := strings.TrimSpace(in.Memo); v != "" {
		fields["memo"] = v
	}
	if v := strings.TrimSpace(in.FromAddress); v != "" {
		fields["from_address"] = v
	}
	if v := strings.TrimSpace(in.ToAddress); v != "" {
		fields["to_address"] = v
	}
	if v := strings.TrimSpace(in.TxHash); v != "" {
		fields["tx_hash"] = v
	}
	if len(fields) == 0 {
		return &pb.UpsertUserTransactionRecordMetaResponse{Success: true, Message: "ok"}, nil
	}
	if err := l.svcCtx.UserTxRepo.UpdateMeta(l.ctx, in.Id, fields); err != nil {
		l.Logger.Errorf("UpsertUserTransactionRecordMeta failed: %v", err)
		return &pb.UpsertUserTransactionRecordMetaResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.UpsertUserTransactionRecordMetaResponse{Success: true, Message: "ok"}, nil
}
