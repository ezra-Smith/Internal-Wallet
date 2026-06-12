package logic

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UpdateDepositLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateDepositLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateDepositLogic {
	return &UpdateDepositLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func canTransitionDeposit(from, to string) bool {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == to {
		return true
	}
	switch from {
	case "pending":
		return to == "confirmed" || to == "failed"
	case "confirmed":
		return to == "completed" || to == "failed"
	default:
		return false
	}
}

func (l *UpdateDepositLogic) UpdateDeposit(in *pb.UpdateDepositRequest) (*pb.UpdateDepositResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id", map[string]string{"id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.WalletDepositRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_NOT_CONFIGURED", "accounting rpc not configured", nil)
	}
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	var updated *model.WalletDepositModel
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var m model.WalletDepositModel
		if err := tx.WithContext(l.ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND deleted_at IS NULL", in.Id).
			First(&m).Error; err != nil {
			return err
		}

		if m.Status == "completed" || m.Status == "failed" {
			return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "IMMUTABLE", "deposit is immutable", map[string]string{"status": m.Status})
		}

		fields := map[string]interface{}{
			"updated_at": &now,
		}
		changes := map[string]interface{}{}

		newStatus := m.Status
		if in.Status != nil {
			v := strings.TrimSpace(in.Status.Value)
			if !validateDepositStatus(v) {
				return errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STATUS", "invalid status", map[string]string{"status": "invalid"})
			}
			newStatus = v
			if !canTransitionDeposit(m.Status, newStatus) {
				return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "INVALID_TRANSITION", "invalid status transition", map[string]string{
					"from": m.Status,
					"to":   newStatus,
				})
			}
			fields["status"] = newStatus
			changes["status"] = newStatus
		}
		if in.Confirmations != nil {
			if in.Confirmations.Value < 0 {
				return errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_CONFIRMATIONS", "invalid confirmations", map[string]string{"confirmations": "invalid"})
			}
			fields["confirmations"] = in.Confirmations.Value
			changes["confirmations"] = in.Confirmations.Value
		}
		if in.BlockNumber != nil {
			if in.BlockNumber.Value <= 0 {
				fields["block_number"] = nil
				changes["block_number"] = 0
			} else {
				fields["block_number"] = in.BlockNumber.Value
				changes["block_number"] = in.BlockNumber.Value
			}
		}
		if len(changes) == 0 {
			return errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "no fields to update", map[string]string{"fields": "empty"})
		}

		// On status transition to "completed", credit the user's available balance via Accounting (idempotent).
		if strings.TrimSpace(newStatus) == "completed" && strings.TrimSpace(m.Status) != "completed" {
			idemKey, bizRef := depositConfirmIdemKeyAndBizRef(m.TransactionHash)
			confirmResp, confirmErr := l.svcCtx.AccountingRpc.ConfirmDeposit(l.ctx, &pb.ConfirmDepositRequest{
				IdempotencyKey: idemKey,
				BizRef:         bizRef,
				UserId:         m.UserID,
				AssetCode:      m.AssetCode,
				ChainCode:      m.ChainCode,
				AmountDecimal:  strings.TrimSpace(m.Amount),
				TxHash:         strings.TrimSpace(m.TransactionHash),
				ToAddress:      strings.TrimSpace(m.DepositAddress),
				Memo: func() string {
					if m.Memo == nil {
						return ""
					}
					return strings.TrimSpace(*m.Memo)
				}(),
			})
			if err := errFromAccountingTx("CONFIRM_DEPOSIT", confirmResp, confirmErr); err != nil {
				return err
			}
		}

		if err := tx.WithContext(l.ctx).
			Model(&model.WalletDepositModel{}).
			Where("id = ? AND deleted_at IS NULL", in.Id).
			Updates(fields).Error; err != nil {
			return err
		}

		var out model.WalletDepositModel
		if err := tx.WithContext(l.ctx).
			Where("id = ? AND deleted_at IS NULL", in.Id).
			First(&out).Error; err != nil {
			return err
		}
		updated = &out

		auditRepo := repository.NewAdminAuditLogRepository(tx)
		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"id":               in.Id,
			"transaction_hash": maskAddress(m.TransactionHash),
			"changes":          changes,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "wallet.deposit.update",
			TargetType:  "wallet_deposit",
			TargetID:    maskAddress(m.TransactionHash),
			Description: "更新充值记录: " + maskAddress(m.TransactionHash),
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		// errx.New returns a gRPC status error; propagate it.
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		if strings.Contains(strings.ToLower(err.Error()), "record not found") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "deposit not found", nil)
		}
		l.Logger.Errorf("update deposit failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "update deposit failed", nil)
	}

	return &pb.UpdateDepositResponse{
		Success: true,
		Message: "ok",
		Data: &pb.UpdateDepositData{
			Deposit: toPBDepositItem(updated),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
