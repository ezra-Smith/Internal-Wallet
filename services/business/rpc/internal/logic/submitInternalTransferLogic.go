package logic

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/common/utils"
	"internalwallet/pkg/notify"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/alert"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/repository"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type SubmitInternalTransferLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSubmitInternalTransferLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubmitInternalTransferLogic {
	return &SubmitInternalTransferLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// resolveTargetUser 解析目标用户标识符（支持 UID、Email），并返回用户对象
// 支持的格式：
//   - UID: 纯数字字符串（如 "1234"）
//   - Email: 包含 @ 的邮箱地址（如 "user@example.com"）
//
// 返回错误类型：
//   - repository.ErrProfileNotFound: 用户不存在
//   - 其他错误: 格式错误或系统错误
func (l *SubmitInternalTransferLogic) resolveTargetUser(raw string) (*model.UserModel, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return nil, errx.InternalTransferInvalidTargetUserFormat()
	}

	// 1. 尝试作为 UID（纯数字）
	if isDigits(v) {
		userID, err := strconv.ParseInt(v, 10, 64)
		if err != nil || userID <= 0 {
			return nil, errx.InternalTransferInvalidTargetUserFormat()
		}
		user, err := l.svcCtx.UserAccountRepository.GetByID(l.ctx, userID)
		if err != nil {
			if errors.Is(err, repository.ErrProfileNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, repository.ErrProfileNotFound // 返回标准错误，让调用方识别
			}
			l.Logger.Errorf("GetByID failed: %v", err)
			return nil, errx.DBError()
		}
		if user == nil {
			return nil, repository.ErrProfileNotFound
		}
		return user, nil
	}

	// 2. 尝试作为 Email（包含 @）
	if strings.Contains(v, "@") {
		email := strings.ToLower(strings.TrimSpace(v))
		if !utils.ValidateEmail(email) {
			return nil, errx.InternalTransferInvalidTargetUserFormat()
		}
		user, err := l.svcCtx.UserAccountRepository.GetByEmail(l.ctx, email)
		if err != nil {
			if errors.Is(err, repository.ErrProfileNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, repository.ErrProfileNotFound // 返回标准错误，让调用方识别
			}
			l.Logger.Errorf("GetByEmail failed: %v", err)
			return nil, errx.DBError()
		}
		if user == nil || user.ID == 0 {
			return nil, repository.ErrProfileNotFound
		}
		return user, nil
	}

	return nil, errx.InternalTransferInvalidTargetUserFormat()
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func (l *SubmitInternalTransferLogic) SubmitInternalTransfer(in *pb.SubmitInternalTransferReq) (*pb.SubmitInternalTransferResp, error) {
	// 1. Validate input
	if in == nil || in.ToUserId == "" || in.AssetCode == "" || in.Amount == "" {
		return nil, errx.InvalidParam("invalid params: to_user_id, asset_code and amount are required")
	}

	// Get current user
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	fromUserID, _ := strconv.ParseInt(uidStr, 10, 64)
	if fromUserID == 0 {
		return nil, errx.Unauthorized("unauthorized")
	}

	// 检查安全冷却期（修改交易密码或解绑GA后24小时内禁止转账）
	cooldownInfo, err := CheckSecurityCooldown(l.ctx, l.svcCtx, fromUserID)
	if err != nil {
		l.Logger.Errorf("Failed to check security cooldown for user %d: %v", fromUserID, err)
		// 降级处理，不阻塞主流程
	} else if cooldownInfo.InCooldown {
		errMsg := GetCooldownErrorMessage(l.ctx, cooldownInfo.Reason, cooldownInfo.RemainingSeconds)
		l.Logger.Infof("User %d in security cooldown (reason=%s), rejecting internal transfer", fromUserID, cooldownInfo.Reason)
		return nil, errx.SecurityCooldownActiveWithMessage(errMsg)
	}

	// 解析目标用户（支持 UID、Email）
	toUser, err := l.resolveTargetUser(strings.TrimSpace(in.ToUserId))
	if err != nil {
		// 检查是否是用户不存在的错误
		if errors.Is(err, repository.ErrProfileNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errx.InternalTransferTargetUserNotFound()
		}
		// 其他错误（格式错误、系统错误等）直接返回
		return nil, err
	}
	if toUser == nil || toUser.ID == 0 {
		return nil, errx.InternalTransferTargetUserNotFound()
	}
	toUserID := toUser.ID

	// 不能转账给自己
	if fromUserID == toUserID {
		return nil, errx.InvalidParam("cannot transfer to yourself")
	}

	// Parse and validate amount
	amountStr := strings.TrimSpace(in.Amount)
	amount, err := decimal.NewFromString(amountStr)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		return nil, errx.InvalidParam("invalid amount: must be positive")
	}

	assetCode := normalizeCode(in.AssetCode)
	if assetCode == "" {
		return nil, errx.InvalidParam("invalid asset_code")
	}

	// 获取资产精度信息，用于格式化金额字符串（避免科学计数法）
	var assetPrecision int32 = 18 // 默认精度
	if l.svcCtx.AccountingRpc != nil {
		accResp, err := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: assetCode})
		if err == nil && accResp != nil && accResp.Item != nil && accResp.Item.Precision > 0 {
			assetPrecision = accResp.Item.Precision
		}
	}
	// 使用 StringFixed 确保格式固定，避免科学计数法和尾部零被去掉的问题
	amountFormatted := amount.StringFixed(assetPrecision)

	// 2. Verify trade password (if required) - 带尝试次数限制
	tradePassword := strings.TrimSpace(in.TradePassword)
	if tradePassword != "" {
		tradePasswordHash, hasTradePassword, err := l.svcCtx.UserAccountRepository.GetTradePasswordHash(l.ctx, fromUserID)
		if err != nil {
			l.Logger.Errorf("Failed to get trade password hash: %v", err)
			return nil, errx.Internal("internal error")
		}
		if hasTradePassword && tradePasswordHash != "" {
			if err := VerifyTradePasswordSimple(l.ctx, l.svcCtx, fromUserID, tradePassword, tradePasswordHash, "internal_transfer"); err != nil {
				return nil, err
			}
		}
	}

	// 3. 目标用户已在 resolveTargetUser 中验证，无需重复查询
	// 4. Check if asset is enabled for transfer (已在上面获取精度时检查过，这里不再重复调用)

	// 5. Check currency settings for transfer feature
	currencySettings, err := l.svcCtx.CurrencySettingsRepository.GetByAssetCode(l.ctx, assetCode)
	if err != nil || currencySettings == nil {
		return nil, errx.InvalidParam("该币种未配置内部转账功能")
	}
	if !currencySettings.Web2TransferEnabled {
		return nil, errx.InvalidParam("该币种已禁用内部转账功能")
	}

	// 6. Determine audit strategy by matching global rules
	// 内部转账只使用全局配置（按资产分组）
	strategy := l.matchGlobalTransferAuditRules(assetCode, amount)

	// 如果策略为空，表示金额不在任何规则范围内，拒绝转账
	if strategy == "" {
		l.Logger.Errorf("Transfer rejected: amount %s not in any audit rule range for asset %s", amount.String(), assetCode)
		return nil, errx.InternalTransferAmountOutOfRange()
	}

	l.Logger.Infof("Matched transfer audit strategy: asset=%s, amount=%s, strategy=%s", assetCode, amount.String(), strategy)

	// 7. Create transfer order
	note := strings.TrimSpace(in.Note)
	if len(note) > 200 {
		note = note[:200]
	}
	var notePtr *string
	if note != "" {
		notePtr = &note
	}

	orderID := utils.GenerateID()
	order := &model.CurrencyTransferOrderModel{
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		AssetCode:  assetCode,
		Amount:     amountFormatted, // 使用固定精度格式化的金额字符串
		Fee:        "0",
		Note:       notePtr,
		Strategy:   strategy,
		Status:     model.TransferStatusPending,
	}
	order.ID = orderID

	if err := l.svcCtx.CurrencyTransferOrderRepository.Create(l.ctx, order); err != nil {
		l.Logger.Errorf("Failed to create transfer order: %v", err)
		return nil, errx.Internal("failed to create transfer order")
	}

	// 8. If auto strategy, execute immediately
	// Normalize strategy to ensure consistent comparison
	strategy = strings.ToLower(strings.TrimSpace(strategy))
	needsAudit := strategy == model.TransferStrategyManualAuto

	l.Logger.Infof("Processing transfer order: orderID=%d, strategy=%s, needsAudit=%v", orderID, strategy, needsAudit)

	if strategy == model.TransferStrategyAuto {
		l.Logger.Infof("Auto strategy detected, executing transfer immediately for orderID=%d", orderID)
		if err := l.executeTransfer(order); err != nil {
			l.Logger.Errorf("Failed to execute transfer: %v", err)
			// Update order status to failed - 使用中文化的错误信息
			errMsg := formatTransferErrorMessage(err)
			order.Status = model.TransferStatusFailed
			order.ErrorMessage = &errMsg
			_ = l.svcCtx.CurrencyTransferOrderRepository.Update(l.ctx, order)
			// 将可识别的业务错误（例如余额不足）映射为明确的业务码，避免前端仅收到"系统错误"
			if isInsufficientFundsErr(err) {
				return nil, errx.InsufficientBalance()
			}
			return nil, errx.Internal("transfer failed: " + err.Error())
		}
		l.Logger.Infof("Transfer executed successfully for orderID=%d", orderID)
	} else {
		// Manual audit required - 需要冻结转出方资金，防止超额转账
		l.Logger.Infof("Manual audit required (strategy=%s), freezing funds for orderID=%d", strategy, orderID)

		// 冻结转出方资金
		freezeResp, err := l.freezeTransferAmount(order)
		if err != nil {
			l.Logger.Errorf("Failed to freeze transfer amount for orderID=%d: %v", orderID, err)
			// 冻结失败，需要将订单标记为失败
			errMsg := formatTransferErrorMessage(err)
			order.Status = model.TransferStatusFailed
			order.ErrorMessage = &errMsg
			_ = l.svcCtx.CurrencyTransferOrderRepository.Update(l.ctx, order)

			if isInsufficientFundsErr(err) {
				return nil, errx.InsufficientBalance()
			}
			return nil, errx.Internal("freeze failed: " + err.Error())
		}

		// 记录冻结交易ID
		if freezeResp != nil && freezeResp.TxId > 0 {
			order.FreezeLedgerTxID = &freezeResp.TxId
		}
		order.Status = model.TransferStatusPending
		order.Strategy = strategy // Ensure strategy is normalized
		_ = l.svcCtx.CurrencyTransferOrderRepository.Update(l.ctx, order)

		l.Logger.Infof("Funds frozen successfully for orderID=%d, freeze_ledger_tx_id=%d", orderID, freezeResp.TxId)

		// 创建 pending 状态的交易记录，让用户能在交易记录中看到待审核的内部转账
		if err := l.createPendingInternalTransferRecord(order); err != nil {
			l.Logger.Errorf("Failed to create pending internal transfer record: %v", err)
			// 不影响主流程，只记录日志
		}
	}

	return &pb.SubmitInternalTransferResp{
		Success:    true,
		Message:    "ok",
		OrderId:    strconv.FormatInt(orderID, 10),
		Status:     order.Status,
		NeedsAudit: needsAudit,
		Strategy:   strategy,
	}, nil
}

// matchGlobalTransferAuditRules matches the amount against global transfer audit rules
// 返回匹配的策略，如果没有匹配到任何规则则返回空字符串（表示拒绝）
func (l *SubmitInternalTransferLogic) matchGlobalTransferAuditRules(assetCode string, amount decimal.Decimal) string {
	// Query global transfer audit rules for this asset from admin service
	if l.svcCtx.AdminRpc == nil {
		l.Logger.Errorf("AdminRpc not available, rejecting transfer")
		return "" // AdminRpc 不可用，拒绝转账
	}

	resp, err := l.svcCtx.AdminRpc.GetCurrencyGlobalTransferAudit(l.ctx, &pb.GetCurrencyGlobalTransferAuditRequest{})
	if err != nil || resp == nil || resp.Data == nil {
		l.Logger.Errorf("GetCurrencyGlobalTransferAudit failed: %v", err)
		return "" // 获取规则失败，拒绝转账
	}

	// Find rules for this asset
	var rules []*pb.CurrencyTransferAuditRuleItem
	for _, group := range resp.Data.AuditRules {
		if strings.EqualFold(group.AssetCode, assetCode) {
			rules = group.Rules
			break
		}
	}

	if len(rules) == 0 {
		// 没有配置规则时，拒绝转账（必须配置规则才能转账）
		l.Logger.Infof("No global transfer audit rules for asset %s, rejecting transfer", assetCode)
		return ""
	}

	// Match rules (sorted by sort_order, first match wins)
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		minAmount, _ := decimal.NewFromString(rule.MinAmount)
		if amount.LessThan(minAmount) {
			continue
		}

		if rule.MaxAmount != "" {
			maxAmount, _ := decimal.NewFromString(rule.MaxAmount)
			if amount.GreaterThanOrEqual(maxAmount) {
				continue
			}
		}

		// Matched!
		matchedStrategy := strings.ToLower(strings.TrimSpace(rule.Strategy))
		l.Logger.Infof("Matched global transfer audit rule for asset %s amount %s: raw_strategy=%s, normalized_strategy=%s", assetCode, amount.String(), rule.Strategy, matchedStrategy)
		return matchedStrategy
	}

	// No rule matched - 有规则但金额不在任何规则范围内，返回空字符串表示拒绝
	l.Infof("No transfer audit rule matched for asset %s amount %s, transfer will be rejected", assetCode, amount.String())
	return "" // 返回空字符串，调用方检查并拒绝转账
}

// executeTransfer calls Accounting RPC to perform the actual transfer
func (l *SubmitInternalTransferLogic) executeTransfer(order *model.CurrencyTransferOrderModel) error {
	if l.svcCtx.AccountingRpc == nil {
		return errx.Internal("accounting service not available")
	}

	// Generate idempotency key
	idempotencyKey := "transfer:" + strconv.FormatInt(order.ID, 10)
	bizRef := "internal_transfer:" + strconv.FormatInt(order.ID, 10)

	resp, err := l.svcCtx.AccountingRpc.Transfer(l.ctx, &pb.TransferRequest{
		IdempotencyKey: idempotencyKey,
		BizRef:         bizRef,
		FromUserId:     order.FromUserID,
		ToUserId:       order.ToUserID,
		FromBucket:     pb.BalanceBucket_BUCKET_AVAILABLE,
		ToBucket:       pb.BalanceBucket_BUCKET_AVAILABLE,
		AssetCode:      order.AssetCode,
		AmountDecimal:  order.Amount,
	})

	if err != nil {
		if isInsufficientFundsErr(err) {
			return errx.InsufficientBalance()
		}
		return err
	}

	if !resp.Success {
		// Accounting 服务在余额不足等情况下通常返回 Success=false + Message
		if isInsufficientFundsMsg(resp.Message) {
			return errx.InsufficientBalance()
		}
		return errx.Internal(resp.Message)
	}

	// Update order status
	now := time.Now()
	order.Status = model.TransferStatusCompleted
	order.LedgerTxID = &resp.TxId
	order.AuditedAt = &now
	if err := l.svcCtx.CurrencyTransferOrderRepository.Update(l.ctx, order); err != nil {
		return err
	}

	// Create transaction records for both users (internal transfer)
	if err := l.createInternalTransferRecords(order, resp.TxId); err != nil {
		// Best-effort: order is already updated; log error but don't fail
		l.Logger.Errorf("Failed to create internal transfer records: %v", err)
	}

	// 异步发送邮件通知给双方（不阻塞响应）
	go l.sendInternalTransferSuccessEmail(order)

	// 交易金额预警检查（异步，不阻塞响应）
	go l.checkInternalTransferAlert(order)

	return nil
}

// checkInternalTransferAlert 检查内部转账金额是否触发预警
func (l *SubmitInternalTransferLogic) checkInternalTransferAlert(order *model.CurrencyTransferOrderModel) {
	amount, err := decimal.NewFromString(strings.TrimSpace(order.Amount))
	if err != nil {
		l.Logger.Errorf("[Alert] Failed to parse internal transfer amount: %v", err)
		return
	}

	alertChecker := alert.NewAlertChecker(l.svcCtx)
	_ = alertChecker.CheckAndNotify(
		l.ctx,
		"internal_transfer", // 内部转账
		strings.TrimSpace(order.AssetCode),
		amount,
		decimal.Zero, // 自动计算 USD 金额
		map[string]interface{}{
			"order_id":     order.ID,
			"from_user_id": order.FromUserID,
			"to_user_id":   order.ToUserID,
			"ledger_tx_id": derefInt64(order.LedgerTxID),
		},
	)
}

func isInsufficientFundsErr(err error) bool {
	if err == nil {
		return false
	}
	// unwrap 一层，兼容 wrapped errors
	for i := 0; i < 3 && err != nil; i++ {
		if isInsufficientFundsMsg(err.Error()) {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}

func isInsufficientFundsMsg(msg string) bool {
	msg = strings.ToLower(strings.TrimSpace(msg))
	if msg == "" {
		return false
	}
	// 覆盖 accounting engine / grpc desc 常见文案
	return strings.Contains(msg, "insufficient funds") ||
		strings.Contains(msg, "insufficient balance") ||
		strings.Contains(msg, "余额不足")
}

// formatTransferErrorMessage 将技术错误信息转换为用户友好的中文错误信息
// 用于存储到数据库的 error_message 字段，展示给用户看
func formatTransferErrorMessage(err error) string {
	if err == nil {
		return "转账失败"
	}
	msg := strings.ToLower(err.Error())

	// 余额不足
	if isInsufficientFundsMsg(msg) {
		return "余额不足"
	}

	// 账户不存在
	if strings.Contains(msg, "account not found") || strings.Contains(msg, "user not found") {
		return "账户不存在"
	}

	// 资产不支持
	if strings.Contains(msg, "asset not found") || strings.Contains(msg, "asset not supported") {
		return "资产不支持"
	}

	// 转账金额无效
	if strings.Contains(msg, "invalid amount") || strings.Contains(msg, "amount must be positive") {
		return "转账金额无效"
	}

	// 精度错误
	if strings.Contains(msg, "precision") || strings.Contains(msg, "decimal") {
		return "金额精度错误"
	}

	// 账户被冻结/禁用
	if strings.Contains(msg, "frozen") || strings.Contains(msg, "disabled") || strings.Contains(msg, "blocked") {
		return "账户已被冻结"
	}

	// 超时
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") {
		return "请求超时，请稍后重试"
	}

	// 网络错误
	if strings.Contains(msg, "connection") || strings.Contains(msg, "network") {
		return "网络错误，请稍后重试"
	}

	// 默认错误
	return "转账失败，请稍后重试"
}

// createPendingInternalTransferRecord 为待审核的内部转账创建一条 pending 状态的交易记录（仅转出方）
// 转入方在审核通过后再创建记录
func (l *SubmitInternalTransferLogic) createPendingInternalTransferRecord(order *model.CurrencyTransferOrderModel) error {
	if l.svcCtx.UserTransactionRecordRepository == nil {
		return errors.New("UserTransactionRecordRepository not initialized")
	}

	bizRef := "internal_transfer:" + strconv.FormatInt(order.ID, 10)
	idempotencyKey := "transfer:" + strconv.FormatInt(order.ID, 10)

	// 转出方记录（pending 状态）
	fromRecord := &model.UserTransactionRecordModel{
		UserId:         order.FromUserID,
		Type:           2, // withdraw（内部转账转出方）
		Asset:          order.AssetCode,
		ChainCode:      "", // 内部转账无链
		Amount:         order.Amount,
		Fee:            order.Fee,
		Status:         model.TransferStatusPending, // pending 状态
		Memo:           derefString(order.Note),
		FromAddress:    "", // 内部转账无地址
		ToAddress:      "", // 内部转账无地址
		TxHash:         "", // 内部转账无链上交易
		BizRef:         bizRef,
		IdempotencyKey: idempotencyKey + ":from",
	}
	fromRecord.ID = utils.GenerateID()

	return l.svcCtx.UserTransactionRecordRepository.CreateRecord(l.ctx, fromRecord)
}

// createInternalTransferRecords 为内部转账创建两条交易记录（转出方和转入方）
func (l *SubmitInternalTransferLogic) createInternalTransferRecords(order *model.CurrencyTransferOrderModel, ledgerTxID int64) error {
	if l.svcCtx.UserTransactionRecordRepository == nil {
		return errors.New("UserTransactionRecordRepository not initialized")
	}

	bizRef := "internal_transfer:" + strconv.FormatInt(order.ID, 10)
	idempotencyKey := "transfer:" + strconv.FormatInt(order.ID, 10)

	// 转出方记录（使用 withdraw 类型）
	fromRecord := &model.UserTransactionRecordModel{
		UserId:           order.FromUserID,
		Type:             2, // withdraw（内部转账转出方）
		Asset:            order.AssetCode,
		ChainCode:        "", // 内部转账无链
		Amount:           order.Amount,
		Fee:              order.Fee,
		Status:           order.Status,
		Memo:             derefString(order.Note),
		FromAddress:      "",          // 内部转账无地址
		ToAddress:        "",          // 内部转账无地址
		TxHash:           "",          // 内部转账无链上交易
		SettleLedgerTxID: &ledgerTxID, // 转出方使用 settle_ledger_tx_id
		BizRef:           bizRef,
		IdempotencyKey:   idempotencyKey + ":from",
	}
	fromRecord.ID = utils.GenerateID()

	// 转入方记录（使用 deposit 类型）
	toRecord := &model.UserTransactionRecordModel{
		UserId:            order.ToUserID,
		Type:              1, // deposit（内部转账转入方）
		Asset:             order.AssetCode,
		ChainCode:         "", // 内部转账无链
		Amount:            order.Amount,
		Fee:               "0", // 转入方无手续费
		Status:            order.Status,
		Memo:              derefString(order.Note),
		FromAddress:       "",          // 内部转账无地址
		ToAddress:         "",          // 内部转账无地址
		TxHash:            "",          // 内部转账无链上交易
		ConfirmLedgerTxID: &ledgerTxID, // 转入方使用 confirm_ledger_tx_id
		BizRef:            bizRef,
		IdempotencyKey:    idempotencyKey + ":to",
	}
	toRecord.ID = utils.GenerateID()

	// 创建两条记录
	if err := l.svcCtx.UserTransactionRecordRepository.CreateRecord(l.ctx, fromRecord); err != nil {
		return err
	}
	if err := l.svcCtx.UserTransactionRecordRepository.CreateRecord(l.ctx, toRecord); err != nil {
		return err
	}

	return nil
}

// freezeTransferAmount 冻结转出方资金（需审核时使用）
// 将转出方的可用余额移动到冻结余额，防止超额转账
func (l *SubmitInternalTransferLogic) freezeTransferAmount(order *model.CurrencyTransferOrderModel) (*pb.LedgerTxResponse, error) {
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.Internal("accounting service not available")
	}

	// 生成幂等键
	idempotencyKey := "transfer_freeze:" + strconv.FormatInt(order.ID, 10)
	bizRef := "internal_transfer_freeze:" + strconv.FormatInt(order.ID, 10)

	resp, err := l.svcCtx.AccountingRpc.FreezeUserAssets(l.ctx, &pb.FreezeUserAssetsRequest{
		IdempotencyKey: idempotencyKey,
		BizRef:         bizRef,
		UserId:         order.FromUserID,
		AssetCode:      order.AssetCode,
		AmountDecimal:  order.Amount,
		Reason:         "internal_transfer_pending_audit",
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || !resp.Success {
		msg := "freeze failed"
		if resp != nil && resp.Message != "" {
			msg = resp.Message
		}
		return nil, errors.New(msg)
	}

	return resp, nil
}

// sendInternalTransferSuccessEmail 发送内部转账成功邮件通知（给双方）
func (l *SubmitInternalTransferLogic) sendInternalTransferSuccessEmail(order *model.CurrencyTransferOrderModel) {
	// 获取转出方和转入方用户信息
	fromUser, err := l.svcCtx.UserAccountRepository.GetByID(context.Background(), order.FromUserID)
	if err != nil || fromUser == nil {
		l.Logger.Errorf("Failed to get from_user info for transfer email: user_id=%d error=%v", order.FromUserID, err)
		// 继续尝试发送给转入方
	}

	toUser, err := l.svcCtx.UserAccountRepository.GetByID(context.Background(), order.ToUserID)
	if err != nil || toUser == nil {
		l.Logger.Errorf("Failed to get to_user info for transfer email: user_id=%d error=%v", order.ToUserID, err)
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
	orderNote := strings.TrimSpace(derefString(order.Note))

	// 格式化转出方和转入方的显示信息（邮箱优先级最高）
	fromUserDisplay := fmt.Sprintf("UID: %d", order.FromUserID)
	if fromUser != nil {
		displayName := ""
		// 邮箱优先级更高
		if fromUser.Email != "" {
			displayName = fromUser.Email
		} else if strings.TrimSpace(fromUser.Nickname) != "" {
			displayName = fromUser.Nickname
		}

		if displayName != "" {
			fromUserDisplay = fmt.Sprintf("%s (UID: %d)", displayName, order.FromUserID)
		}
	}

	// 格式化接收账户：邮箱 (UID: xxx)
	toUserDisplay := fmt.Sprintf("UID: %d", order.ToUserID)
	if toUser != nil {
		displayName := ""
		// 邮箱优先级更高
		if toUser.Email != "" {
			displayName = toUser.Email
		} else if strings.TrimSpace(toUser.Nickname) != "" {
			displayName = toUser.Nickname
		}

		if displayName != "" {
			toUserDisplay = fmt.Sprintf("%s (UID: %d)", displayName, order.ToUserID)
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
		fromUID := fmt.Sprintf("%d", order.FromUserID)

		err = notify.SendInternalTransferSuccessEmailAsync(
			fromUser.Email,
			fromUsername,
			fromUID,
			order.AssetCode,
			notify.FormatAmount(order.Amount), // 格式化金额显示
			transferTime,
			fromUserDisplay, // FromAccount
			toUserDisplay,   // ToAccount
			toReceiverNote,  // ReceiverNote (接收方昵称)
			orderID,
			orderNote, // OrderNote (转账备注，从 order.Note 获取)
		)

		if err != nil {
			l.Logger.Errorf("Failed to send transfer email to from_user %s: %v", fromUser.Email, err)
		} else {
			l.Logger.Infof("Transfer success email sent to from_user %s", fromUser.Email)
		}
	}

	// 发送给转入方
	if toUser != nil {
		toUsername := strings.TrimSpace(toUser.Nickname)
		if toUsername == "" {
			toUsername = toUser.Email
		}
		toUID := fmt.Sprintf("%d", order.ToUserID)

		err = notify.SendInternalTransferSuccessEmailAsync(
			toUser.Email,
			toUsername,
			toUID,
			order.AssetCode,
			notify.FormatAmount(order.Amount), // 格式化金额显示
			transferTime,
			fromUserDisplay, // FromAccount
			toUserDisplay,   // ToAccount
			toReceiverNote,  // ReceiverNote (接收方昵称)
			orderID,
			orderNote, // OrderNote (转账备注，从 order.Note 获取)
		)

		if err != nil {
			l.Logger.Errorf("Failed to send transfer email to to_user %s: %v", toUser.Email, err)
		} else {
			l.Logger.Infof("Transfer success email sent to to_user %s", toUser.Email)
		}
	}
}
