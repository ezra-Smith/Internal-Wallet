package logic

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"

	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func sumAmountAndFee(amountStr, feeStr string) (string, error) {
	amountStr = strings.TrimSpace(amountStr)
	feeStr = strings.TrimSpace(feeStr)
	if feeStr == "" {
		feeStr = "0"
	}
	amt, err := decimal.NewFromString(amountStr)
	if err != nil {
		return "", err
	}
	fee, err := decimal.NewFromString(feeStr)
	if err != nil {
		return "", err
	}
	return amt.Add(fee).String(), nil
}

func depositConfirmIdemKeyAndBizRef(txHash string) (string, string) {
	txHash = strings.TrimSpace(txHash)
	sum := sha256.Sum256([]byte(txHash))
	hashHex := hex.EncodeToString(sum[:])
	return "deposit:confirm:" + hashHex, "deposit_tx:" + hashHex
}

func transferBatchCreditIdemKeyAndBizRef(batchID string, transferID string, transferType string) (string, string) {
	batchID = strings.TrimSpace(batchID)
	transferID = strings.TrimSpace(transferID)
	transferType = strings.ToLower(strings.TrimSpace(transferType))
	if transferType != "airdrop" {
		transferType = "normal"
	}

	sum := sha256.Sum256([]byte("transfer_batch|" + batchID + "|" + transferID))
	hashHex := hex.EncodeToString(sum[:])
	return "transfer_batch:credit:" + hashHex, "transfer_batch:" + transferType + ":" + batchID + ":" + transferID
}

func errFromAccountingTx(action string, resp *pb.LedgerTxResponse, callErr error) error {
	if callErr != nil {
		if st, ok := status.FromError(callErr); ok {
			grpcCode := st.Code()
			httpStatus := 500
			bizCode := errx.CodeInternalError
			switch grpcCode {
			case codes.InvalidArgument:
				httpStatus = 400
				bizCode = errx.CodeInvalidParam
			case codes.Unauthenticated:
				httpStatus = 401
				bizCode = errx.CodeUnauthorized
			case codes.PermissionDenied:
				httpStatus = 403
				bizCode = errx.CodeForbidden
			case codes.NotFound:
				httpStatus = 404
				bizCode = errx.CodeNotFound
			case codes.FailedPrecondition, codes.Aborted, codes.AlreadyExists:
				httpStatus = 409
				bizCode = errx.CodeConflict
			case codes.Unavailable:
				httpStatus = 503
			case codes.DeadlineExceeded:
				httpStatus = 504
			}
			return errx.New(grpcCode, httpStatus, bizCode, action+"_RPC_ERROR", strings.TrimSpace(st.Message()), nil)
		}
		return errx.New(codes.Internal, 500, errx.CodeInternalError, action+"_RPC_ERROR", strings.TrimSpace(callErr.Error()), nil)
	}
	if resp == nil {
		return errx.New(codes.Internal, 500, errx.CodeInternalError, action+"_RPC_EMPTY", "accounting rpc empty response", nil)
	}
	if resp.Success {
		return nil
	}

	msg := strings.TrimSpace(resp.Message)
	if msg == "" {
		msg = "accounting failed"
	}
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "insufficient funds"):
		return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "INSUFFICIENT_FUNDS", msg, nil)
	case strings.Contains(lower, "idempotency conflict"):
		return errx.New(codes.Aborted, 409, errx.CodeConflict, "IDEMPOTENCY_CONFLICT", msg, nil)
	case strings.Contains(lower, "asset not available"):
		return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "ASSET_NOT_AVAILABLE", msg, nil)
	case strings.Contains(lower, "invalid"):
		return errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "ACCOUNTING_INVALID", msg, nil)
	default:
		return errx.New(codes.Internal, 500, errx.CodeInternalError, action+"_FAILED", msg, nil)
	}
}

// toAdminUserTransactionRecordItem 转换用户交易记录项为 Admin 格式
func toAdminUserTransactionRecordItem(item *pb.UserTransactionRecordItem) *pb.AccountingUserTransactionRecordItem {
	if item == nil {
		return nil
	}
	return &pb.AccountingUserTransactionRecordItem{
		Id:                item.Id,
		UserId:            item.UserId,
		TxType:            item.TxType,
		AssetCode:         item.AssetCode,
		ChainCode:         item.ChainCode,
		AmountDecimal:     item.AmountDecimal,
		FeeDecimal:        item.FeeDecimal,
		Status:            item.Status,
		Memo:              item.Memo,
		FromAddress:       item.FromAddress,
		ToAddress:         item.ToAddress,
		TxHash:            item.TxHash,
		Timestamp:         item.Timestamp,
		FreezeLedgerTxId:  item.FreezeLedgerTxId,
		SettleLedgerTxId:  item.SettleLedgerTxId,
		ConfirmLedgerTxId: item.ConfirmLedgerTxId,
		BizRef:            item.BizRef,
		IdempotencyKey:    item.IdempotencyKey,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}
