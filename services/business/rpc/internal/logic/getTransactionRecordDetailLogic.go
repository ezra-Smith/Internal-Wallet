package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetTransactionRecordDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTransactionRecordDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTransactionRecordDetailLogic {
	return &GetTransactionRecordDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetTransactionRecordDetailLogic) GetTransactionRecordDetail(in *pb.GetTransactionRecordDetailReq) (*pb.GetTransactionRecordDetailResp, error) {
	if in == nil || in.Id == "" {
		return nil, errx.InvalidParam("invalid params")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, _ := strconv.ParseInt(uidStr, 10, 64)

	id, err := strconv.ParseInt(in.Id, 10, 64)
	if err != nil || id <= 0 || uid <= 0 {
		return nil, errx.InvalidParam("invalid id")
	}

	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.ServiceNotAvailable("accounting rpc")
	}
	accResp, err := l.svcCtx.AccountingRpc.GetUserTransactionRecordDetail(l.ctx, &pb.GetUserTransactionRecordDetailRequest{Id: id})
	if err != nil || accResp == nil || !accResp.Success || accResp.Item == nil {
		return nil, errx.RecordNotFound()
	}

	item := accResp.Item
	if item.UserId != uid {
		return nil, errx.RecordNotFound()
	}

	counterparty := ""
	switch item.TxType {
	case 1:
		counterparty = strings.TrimSpace(item.FromAddress)
	case 2:
		counterparty = strings.TrimSpace(item.ToAddress)
	}
	txID := strings.TrimSpace(item.BizRef)
	if txID == "" {
		txID = strings.TrimSpace(item.IdempotencyKey)
	}

	// 构建区块链浏览器链接
	chainCode := strings.TrimSpace(item.ChainCode)
	txHash := strings.TrimSpace(item.TxHash)
	explorerURL := BuildExplorerURL(chainCode, txHash)

	return &pb.GetTransactionRecordDetailResp{
		Success:      true,
		Type:         pb.TransactionType(item.TxType),
		Asset:        strings.TrimSpace(item.AssetCode),
		Amount:       strings.TrimSpace(item.AmountDecimal),
		Timestamp:    item.Timestamp,
		Status:       strings.TrimSpace(item.Status),
		Counterparty: counterparty,
		Network:      chainCode,
		TxId:         txID,
		TxHash:       txHash,
		Fee:          strings.TrimSpace(item.FeeDecimal),
		Memo:         strings.TrimSpace(item.Memo),
		ExplorerUrl:  explorerURL,
	}, nil
}
