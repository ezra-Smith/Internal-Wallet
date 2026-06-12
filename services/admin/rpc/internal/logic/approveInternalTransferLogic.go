package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/common/utils"
	"internalwallet/pkg/notify"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/alert"
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

type ApproveInternalTransferLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApproveInternalTransferLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApproveInternalTransferLogic {
	return &ApproveInternalTransferLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ApproveInternalTransferLogic) ApproveInternalTransfer(in *pb.ApproveInternalTransferRequest) (*pb.ApproveInternalTransferResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id", map[string]string{"id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.CurrencyTransferOrderRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_NOT_CONFIGURED", "accounting rpc not configured", nil)
	}
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}
	if err := verifyAdminTwoFA(l.svcCtx, l.ctx, current.ID, in.TwoFaCode); err != nil {
		return nil, err
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	note := strings.TrimSpace(in.AuditNote)

	var updated *model.CurrencyTransferOrderModel
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var m model.CurrencyTransferOrderModel
		if err := tx.WithContext(l.ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", in.Id).
			First(&m).Error; err != nil {
			return err
		}
		if m.Status != model.TransferStatusPending {
			return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "NOT_PENDING", "only pending transfers can be approved", map[string]string{"status": m.Status})
		}
		strategy := strings.ToLower(strings.TrimSpace(m.Strategy))
		if strategy != model.TransferStrategyManualAuto {
			return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "NOT_MANUAL", "only manual transfers can be approved", map[string]string{"strategy": strings.TrimSpace(m.Strategy)})
		}

		// Execute the transfer via Accounting RPC
		idempotencyKey := "transfer:" + strconv.FormatInt(m.ID, 10)
		bizRef := "internal_transfer:" + strconv.FormatInt(m.ID, 10)

		// 判断资金是否已冻结：如果 FreezeLedgerTxID 存在，说明资金在 locked bucket
		fromBucket := pb.BalanceBucket_BUCKET_AVAILABLE
		if m.FreezeLedgerTxID != nil && *m.FreezeLedgerTxID > 0 {
			fromBucket = pb.BalanceBucket_BUCKET_LOCKED
			l.Logger.Infof("Transfer has frozen funds (freeze_tx=%d), using LOCKED bucket", *m.FreezeLedgerTxID)
		}

		transferResp, transferErr := l.svcCtx.AccountingRpc.Transfer(l.ctx, &pb.TransferRequest{
			IdempotencyKey:     idempotencyKey,
			BizRef:             bizRef,
			FromUserId:         m.FromUserID,
			ToUserId:           m.ToUserID,
			FromBucket:         fromBucket,
			ToBucket:           pb.BalanceBucket_BUCKET_AVAILABLE,
			AssetCode:          m.AssetCode,
			AmountDecimal:      m.Amount,
			SkipPrecisionCheck: true, // Skip precision check for admin approval
		})

		if transferErr != nil {
			errMsg := transferErr.Error()
			if err := tx.WithContext(l.ctx).
				Model(&model.CurrencyTransferOrderModel{}).
				Where("id = ?", in.Id).
				Updates(map[string]interface{}{
					"status":        model.TransferStatusFailed,
					"error_message": &errMsg,
					"updated_by":    current.ID,
				}).Error; err != nil {
				return err
			}
			return transferErr
		}

		if transferResp == nil || !transferResp.Success {
			errMsg := "transfer failed"
			if transferResp != nil && transferResp.Message != "" {
				errMsg = transferResp.Message
			}
			if err := tx.WithContext(l.ctx).
				Model(&model.CurrencyTransferOrderModel{}).
				Where("id = ?", in.Id).
				Updates(map[string]interface{}{
					"status":        model.TransferStatusFailed,
					"error_message": &errMsg,
					"updated_by":    current.ID,
				}).Error; err != nil {
				return err
			}
			return fmt.Errorf("transfer failed: %s", errMsg)
		}

		// Update order status to completed
		if err := tx.WithContext(l.ctx).
			Model(&model.CurrencyTransferOrderModel{}).
			Where("id = ?", in.Id).
			Updates(map[string]interface{}{
				"status":         model.TransferStatusCompleted,
				"ledger_tx_id":   transferResp.TxId,
				"audit_admin_id": current.ID,
				"audit_note":     strPtrOrNilTrim(note),
				"audited_at":     &now,
				"updated_by":     current.ID,
			}).Error; err != nil {
			return err
		}

		// Create transaction records directly in Admin service
		// 注意：此时订单状态已更新为 completed，但 m 对象还是旧状态，需要传递 completed 状态
		if l.svcCtx.UserTransactionRecordRepo != nil {
			if err := createInternalTransferRecords(l.ctx, l.svcCtx.UserTransactionRecordRepo, &m, transferResp.TxId, model.TransferStatusCompleted); err != nil {
				l.Logger.Errorf("Failed to create internal transfer records: %v", err)
				// Log error but don't fail the transaction (best-effort)
			}
		} else {
			l.Logger.Infof("UserTransactionRecordRepo not initialized, skipping transaction records creation")
		}

		var out model.CurrencyTransferOrderModel
		if err := tx.WithContext(l.ctx).
			Where("id = ?", in.Id).
			First(&out).Error; err != nil {
			return err
		}
		updated = &out

		// Create audit log
		auditRepo := repository.NewAdminAuditLogRepository(tx)
		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"id":           in.Id,
			"from_user_id": m.FromUserID,
			"to_user_id":   m.ToUserID,
			"asset_code":   m.AssetCode,
			"amount":       m.Amount,
			"strategy":     strings.TrimSpace(m.Strategy),
			"status_from":  m.Status,
			"status_to":    model.TransferStatusCompleted,
			"audit_note":   note,
			"ledger_tx_id": transferResp.TxId,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "currency.transfer.approve",
			TargetType:  "currency_transfer_order",
			TargetID:    strconv.FormatInt(in.Id, 10),
			Description: "审核通过内部转账",
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})

		return nil
	}); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "transfer not found", nil)
		}
		l.Logger.Errorf("approve internal transfer failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "approve transfer failed", nil)
	}

	// 事务成功提交后，发送邮件通知给双方
	// 使用独立的 goroutine 和 context.Background() 避免阻塞响应
	if updated != nil {
		go func(order *model.CurrencyTransferOrderModel) {
			// 使用 context.Background() 避免请求结束后 context 被取消
			sendInternalTransferSuccessEmailFromAdmin(context.Background(), l.svcCtx, order.FromUserID, order.ToUserID, order.AssetCode, order.Amount, order)
		}(updated)

		// 触发预警检查（异步执行，不阻塞响应）
		go func(order *model.CurrencyTransferOrderModel) {
			_ = alert.CheckAndNotifyForTransfer(context.Background(), l.svcCtx, order)
		}(updated)
	}

	return &pb.ApproveInternalTransferResponse{
		Success: true,
		Message: "ok",
		Data: &pb.ApproveInternalTransferData{
			Transfer: toPBInternalTransferItem(updated),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

// createInternalTransferRecords 为内部转账创建/更新交易记录（转出方和转入方）
// 转出方：如果已存在 pending 记录则更新为 completed，否则创建新记录
// 转入方：始终创建新记录（因为转入方记录只在审核通过后才创建）
func createInternalTransferRecords(ctx context.Context, repo repository.UserTransactionRecordRepository, order *model.CurrencyTransferOrderModel, ledgerTxID int64, recordStatus string) error {
	if repo == nil {
		return status.Error(codes.Internal, "UserTransactionRecordRepository not initialized")
	}

	bizRef := "internal_transfer:" + strconv.FormatInt(order.ID, 10)
	idempotencyKey := "transfer:" + strconv.FormatInt(order.ID, 10)

	// 转出方：先尝试更新已有的 pending 记录
	fromIdempotencyKey := idempotencyKey + ":from"
	rowsAffected, err := repo.UpdateStatusByIdempotencyKey(ctx, fromIdempotencyKey, recordStatus, &ledgerTxID)
	if err != nil {
		return err
	}

	// 如果没有找到已有记录，则创建新记录
	if rowsAffected == 0 {
		fromRecord := &model.UserTransactionRecordModel{
			UserId:    order.FromUserID,
			Type:      2, // withdraw（内部转账转出方）
			Asset:     order.AssetCode,
			ChainCode: "", // 内部转账无链
			Amount:    order.Amount,
			Fee:       order.Fee,
			Status:    recordStatus,
			Memo: func() string {
				if order.Note != nil {
					return *order.Note
				}
				return ""
			}(),
			FromAddress:      "",
			ToAddress:        "",
			TxHash:           "",
			SettleLedgerTxID: &ledgerTxID,
			BizRef:           bizRef,
			IdempotencyKey:   fromIdempotencyKey,
		}
		fromRecord.ID = utils.GenerateID()
		if err := repo.CreateRecord(ctx, fromRecord); err != nil {
			return err
		}
	}

	// 转入方记录（始终创建新记录）
	toRecord := &model.UserTransactionRecordModel{
		UserId:    order.ToUserID,
		Type:      1, // deposit（内部转账转入方）
		Asset:     order.AssetCode,
		ChainCode: "", // 内部转账无链
		Amount:    order.Amount,
		Fee:       "0",
		Status:    recordStatus,
		Memo: func() string {
			if order.Note != nil {
				return *order.Note
			}
			return ""
		}(),
		FromAddress:       "",
		ToAddress:         "",
		TxHash:            "",
		ConfirmLedgerTxID: &ledgerTxID,
		BizRef:            bizRef,
		IdempotencyKey:    idempotencyKey + ":to",
	}
	toRecord.ID = utils.GenerateID()

	if err := repo.CreateRecord(ctx, toRecord); err != nil {
		return err
	}

	return nil
}

// sendInternalTransferSuccessEmailFromAdmin 从 admin 服务发送内部转账成功邮件通知（给双方）
func sendInternalTransferSuccessEmailFromAdmin(ctx context.Context, svcCtx *svc.ServiceContext, fromUserID, toUserID int64, assetCode, amount string, order *model.CurrencyTransferOrderModel) {
	if svcCtx.UserRepo == nil {
		logx.Errorf("UserRepo not available, cannot fetch user info for transfer email")
		return
	}

	// 获取转出方用户信息
	fromUser, err := svcCtx.UserRepo.FindByID(ctx, fromUserID)
	if err != nil || fromUser == nil {
		logx.Errorf("Failed to get from_user info for transfer email: user_id=%d error=%v", fromUserID, err)
		// 继续尝试发送给转入方
	}

	// 获取转入方用户信息
	toUser, err := svcCtx.UserRepo.FindByID(ctx, toUserID)
	if err != nil || toUser == nil {
		logx.Errorf("Failed to get to_user info for transfer email: user_id=%d error=%v", toUserID, err)
		// 如果两个都失败就返回
		if fromUser == nil {
			return
		}
	}

	// 格式化时间（UTC）
	transferTime := time.Now().UTC().Format("2006-01-02 15:04:05")
	if order.AuditedAt != nil {
		transferTime = order.AuditedAt.UTC().Format("2006-01-02 15:04:05")
	}

	orderID := fmt.Sprintf("INT%d", order.ID)

	// 获取转账备注
	orderNote := ""
	if order.Note != nil {
		orderNote = strings.TrimSpace(*order.Note)
	}

	// 格式化转出方和转入方的显示信息（邮箱优先级最高）
	fromUserDisplay := fmt.Sprintf("UID: %d", fromUserID)
	if fromUser != nil {
		displayName := ""
		// 邮箱优先级更高
		if fromUser.Email != "" {
			displayName = fromUser.Email
		} else if strings.TrimSpace(fromUser.Nickname) != "" {
			displayName = fromUser.Nickname
		}

		if displayName != "" {
			fromUserDisplay = fmt.Sprintf("%s (UID: %d)", displayName, fromUserID)
		}
	}

	// 格式化接收账户：邮箱 (UID: xxx)
	toUserDisplay := fmt.Sprintf("UID: %d", toUserID)
	if toUser != nil {
		displayName := ""
		// 邮箱优先级更高
		if toUser.Email != "" {
			displayName = toUser.Email
		} else if strings.TrimSpace(toUser.Nickname) != "" {
			displayName = toUser.Nickname
		}

		if displayName != "" {
			toUserDisplay = fmt.Sprintf("%s (UID: %d)", displayName, toUserID)
		}
	}

	// ReceiverNote 格式：昵称 或 昵称/备注：xxx
	toReceiverNote := ""
	if toUser != nil && strings.TrimSpace(toUser.Nickname) != "" {
		toReceiverNote = strings.TrimSpace(toUser.Nickname)
		// 如果有转账备注，拼接上
		if order.Note != nil && strings.TrimSpace(*order.Note) != "" {
			toReceiverNote += "/备注：" + strings.TrimSpace(*order.Note)
		}
	} else if order.Note != nil && strings.TrimSpace(*order.Note) != "" {
		// 如果没有昵称但有备注，只显示备注
		toReceiverNote = "备注：" + strings.TrimSpace(*order.Note)
	}

	// 发送给转出方
	if fromUser != nil {
		fromUsername := strings.TrimSpace(fromUser.Nickname)
		if fromUsername == "" {
			fromUsername = fromUser.Email
		}
		fromUID := fmt.Sprintf("%d", fromUserID)

		err = notify.SendInternalTransferSuccessEmailAsync(
			fromUser.Email,
			fromUsername,
			fromUID,
			assetCode,
			notify.FormatAmount(amount), // 格式化金额显示
			transferTime,
			fromUserDisplay, // FromAccount
			toUserDisplay,   // ToAccount
			toReceiverNote,  // ReceiverNote (接收方昵称)
			orderID,
			orderNote, // OrderNote (转账备注，从 order.Note 获取)
		)

		if err != nil {
			logx.Errorf("Failed to send transfer email to from_user %s: %v", fromUser.Email, err)
		} else {
			logx.Infof("Transfer success email sent to from_user %s", fromUser.Email)
		}
	}

	// 发送给转入方
	if toUser != nil {
		toUsername := strings.TrimSpace(toUser.Nickname)
		if toUsername == "" {
			toUsername = toUser.Email
		}
		toUID := fmt.Sprintf("%d", toUserID)

		err = notify.SendInternalTransferSuccessEmailAsync(
			toUser.Email,
			toUsername,
			toUID,
			assetCode,
			notify.FormatAmount(amount), // 格式化金额显示
			transferTime,
			fromUserDisplay, // FromAccount
			toUserDisplay,   // ToAccount
			toReceiverNote,  // ReceiverNote (接收方昵称)
			orderID,
			orderNote, // OrderNote (转账备注，从 order.Note 获取)
		)

		if err != nil {
			logx.Errorf("Failed to send transfer email to to_user %s: %v", toUser.Email, err)
		} else {
			logx.Infof("Transfer success email sent to to_user %s", toUser.Email)
		}
	}
}
