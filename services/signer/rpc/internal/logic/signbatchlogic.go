package logic

import (
	"context"

	"internalwallet/common/constants"
	"internalwallet/common/errcode"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SignBatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSignBatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SignBatchLogic {
	return &SignBatchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SignBatchLogic) SignBatch(in *pb.SignBatchRequest) (*pb.SignBatchResponse, error) {
	// 参数验证
	if in.BatchId == "" {
		return &pb.SignBatchResponse{
			Code:    int32(errcode.SignerInvalidParams),
			Message: "batch_id is required",
		}, nil
	}
	if in.Chain == "" {
		return &pb.SignBatchResponse{
			Code:    int32(errcode.SignerInvalidParams),
			Message: "chain is required",
		}, nil
	}
	if len(in.Transactions) == 0 {
		return &pb.SignBatchResponse{
			Code:    int32(errcode.SignerInvalidParams),
			Message: "transactions cannot be empty",
		}, nil
	}
	if len(in.Transactions) > 100 {
		return &pb.SignBatchResponse{
			Code:    int32(errcode.SignerBatchTooLarge),
			Message: "batch size exceeds maximum (100)",
		}, nil
	}

	// 批量签名
	results := make([]*pb.SignResultItem, 0, len(in.Transactions))
	successCount := 0
	failedCount := 0

	signLogic := NewSignTransactionLogic(l.ctx, l.svcCtx)

	for _, tx := range in.Transactions {
		// 构造签名请求
		signReq := &pb.SignTransactionRequest{
			RequestId:      tx.RequestId,
			Chain:          in.Chain,
			FromAddress:    tx.FromAddress, // 根据 from_address 自动查询 seed 和派生路径
			RawTransaction: tx.RawTransaction,
			OperationType:  in.OperationType,
			Amount:         tx.Amount,
			ToAddress:      tx.ToAddress,
			AssetSymbol:    tx.AssetSymbol,
			TokenContract:  tx.TokenContract,
		}

		// 调用单个签名逻辑
		signResp, err := signLogic.SignTransaction(signReq)

		resultItem := &pb.SignResultItem{
			RequestId: tx.RequestId,
		}

		if err != nil || signResp.Code != 0 {
			resultItem.Status = constants.SignStatusFailed
			if err != nil {
				resultItem.ErrorMsg = err.Error()
			} else {
				resultItem.ErrorMsg = signResp.Message
			}
			failedCount++
		} else {
			resultItem.Status = constants.SignStatusSuccess
			resultItem.Signature = signResp.Signature
			resultItem.TxHash = signResp.TxHash
			successCount++
		}

		results = append(results, resultItem)
	}

	l.Logger.Infof("✓ Batch signing completed: batch_id=%s, total=%d, success=%d, failed=%d",
		in.BatchId, len(in.Transactions), successCount, failedCount)

	return &pb.SignBatchResponse{
		Code:    0,
		Message: "success",
		BatchId: in.BatchId,
		Total:   int32(len(in.Transactions)),
		Success: int32(successCount),
		Failed:  int32(failedCount),
		Results: results,
	}, nil
}
