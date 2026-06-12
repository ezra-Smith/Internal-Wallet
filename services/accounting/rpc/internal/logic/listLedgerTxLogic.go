package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/repository"
	"internalwallet/services/accounting/rpc/internal/svc"

	"internalwallet/pkg/accounting"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListLedgerTxLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListLedgerTxLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLedgerTxLogic {
	return &ListLedgerTxLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListLedgerTxLogic) ListLedgerTx(in *pb.ListLedgerTxRequest) (*pb.ListLedgerTxResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.ListLedgerTxResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.LedgerRepo == nil {
		return &pb.ListLedgerTxResponse{Success: false, Message: "repository not initialized"}, nil
	}

	var page, pageSize int32 = 1, 20
	var txID, userID int64
	var opType, bizRef, idempotencyKey, assetCode, createdFromStr, createdToStr string
	if in != nil {
		page = in.Page
		pageSize = in.PageSize
		txID = in.TxId
		userID = in.UserId
		opType = strings.TrimSpace(in.OpType)
		bizRef = strings.TrimSpace(in.BizRef)
		idempotencyKey = strings.TrimSpace(in.IdempotencyKey)
		assetCode = accounting.NormalizeAssetCode(in.AssetCode)
		createdFromStr = strings.TrimSpace(in.CreatedFrom)
		createdToStr = strings.TrimSpace(in.CreatedTo)
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	createdFrom, err := parseDateStart(createdFromStr)
	if err != nil {
		return &pb.ListLedgerTxResponse{Success: false, Message: "invalid created_from"}, nil
	}
	createdTo, err := parseDateEnd(createdToStr)
	if err != nil {
		return &pb.ListLedgerTxResponse{Success: false, Message: "invalid created_to"}, nil
	}

	f := repository.LedgerTxListFilter{
		TxID:           txID,
		OpType:         opType,
		BizRef:         bizRef,
		IdempotencyKey: idempotencyKey,
		AssetCode:      assetCode,
		UserID:         userID,
		CreatedFrom:    createdFrom,
		CreatedTo:      createdTo,
	}

	rows, total, err := l.svcCtx.LedgerRepo.ListTx(l.ctx, page, pageSize, f)
	if err != nil {
		l.Logger.Errorf("ListLedgerTx failed: %v", err)
		return &pb.ListLedgerTxResponse{Success: false, Message: err.Error()}, nil
	}

	items := make([]*pb.LedgerTxItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, &pb.LedgerTxItem{
			TxId:           r.ID,
			CreatedAt:      formatTime(r.CreatedAt),
			OpType:         strings.TrimSpace(r.OpType),
			BizRef:         strings.TrimSpace(r.BizRef),
			IdempotencyKey: strings.TrimSpace(r.IdempotencyKey),
			RequestHash:    strings.TrimSpace(r.RequestHash),
		})
	}

	return &pb.ListLedgerTxResponse{
		Success: true,
		Message: "ok",
		Total:   total,
		Items:   items,
	}, nil
}

func parseDateStart(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		return nil, err
	}
	out := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return &out, nil
}

func parseDateEnd(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		return nil, err
	}
	out := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.UTC)
	return &out, nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
