package logic

import (
	"context"
	"encoding/json"
	"fmt"
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
	"gorm.io/gorm"
)

type CreateVaultAdjustmentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateVaultAdjustmentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateVaultAdjustmentLogic {
	return &CreateVaultAdjustmentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateVaultAdjustmentLogic) CreateVaultAdjustment(in *pb.CreateVaultAdjustmentRequest) (*pb.CreateVaultAdjustmentResponse, error) {
	if in == nil {
		in = &pb.CreateVaultAdjustmentRequest{}
	}
	if l.svcCtx.DB == nil ||
		l.svcCtx.AccountingRpc == nil ||
		l.svcCtx.CurrencyChainSettingsRepo == nil ||
		l.svcCtx.VaultNetworkRepo == nil ||
		l.svcCtx.VaultBalanceRepo == nil ||
		l.svcCtx.VaultAdjustmentRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	if in.ChainId == 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_NETWORK", "invalid chain_id", map[string]string{"chain_id": "required"})
	}

	network, err := l.svcCtx.VaultNetworkRepo.FindByChainID(l.ctx, in.ChainId)
	if err != nil || network == nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_NETWORK", "invalid network", map[string]string{"chain_id": "invalid"})
	}

	currency := strings.TrimSpace(in.Currency)
	if currency == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_CURRENCY", "currency required", map[string]string{"currency": "required"})
	}
	assetCode := normalizeCode(currency)
	accAssetResp, callErr := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: assetCode})
	if callErr != nil {
		l.Logger.Errorf("call accounting GetAsset failed: %v", callErr)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting service error", nil)
	}
	if accAssetResp == nil || !accAssetResp.Success || accAssetResp.Item == nil || accAssetResp.Item.Status != 1 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_CURRENCY", "invalid currency", map[string]string{"currency": "invalid"})
	}
	asset := accAssetResp.Item
	chainCode := chainTypeToChainCode(network.ChainType)
	settings, err := l.svcCtx.CurrencyChainSettingsRepo.ListByAssetCode(l.ctx, assetCode)
	if err != nil {
		l.Logger.Errorf("list currency chain settings failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	var mapping *model.CurrencyChainSettingsModel
	for _, s := range settings {
		if s == nil || s.IsDeleted() {
			continue
		}
		if normalizeCode(s.ChainCode) == chainCode {
			mapping = s
			break
		}
	}
	if mapping == nil || mapping.Status != 1 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_CURRENCY", "currency not supported on this network", map[string]string{"currency": "chain_mismatch"})
	}

	adjType := strings.TrimSpace(in.AdjustmentType)
	if !validateVaultAdjustmentType(adjType) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ADJUSTMENT_TYPE", "invalid adjustment_type", map[string]string{"adjustment_type": "invalid"})
	}

	amountRes, err := parseAmountToRaw(in.Amount, asset.Precision, false)
	if err != nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_AMOUNT", "invalid amount", map[string]string{"amount": err.Error()})
	}

	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_REASON", "reason required", map[string]string{"reason": "required"})
	}
	if len(reason) > 255 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_REASON", "reason too long", map[string]string{"reason": "too long"})
	}

	source := strings.TrimSpace(in.Source)
	if !validateVaultAdjustmentSource(source) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_SOURCE", "invalid source", map[string]string{"source": "invalid"})
	}

	sourceAddr := strings.TrimSpace(in.SourceAddress)
	var sourceAddrPtr *string
	if sourceAddr != "" {
		if err := validateChainAddress(network.ChainType, sourceAddr); err != nil {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_SOURCE_ADDRESS", "invalid source_address", map[string]string{"source_address": "invalid"})
		}
		sourceAddrPtr = &sourceAddr
	}

	txHash := strings.TrimSpace(in.TxHash)
	var txHashPtr *string
	if txHash != "" {
		if len(txHash) > 255 {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_TX_HASH", "tx_hash too long", map[string]string{"tx_hash": "too long"})
		}
		txHashPtr = &txHash
	}

	attachmentsBytes, err := marshalVaultAttachments(in.Attachments)
	if err != nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ATTACHMENTS", "invalid attachments", map[string]string{"attachments": "invalid"})
	}

	exists, err := l.svcCtx.VaultAdjustmentRepo.ExistsPendingByNetworkAndCurrency(l.ctx, network.ID, assetCode)
	if err != nil {
		l.Logger.Errorf("check pending adjustment exists failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	if exists {
		return nil, errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "PENDING_ADJUSTMENT_EXISTS", "pending adjustment exists", nil)
	}

	// For decrease, enforce available balance (balance - locked_pending_decrease >= amount).
	contractAddress := ""
	if mapping != nil && mapping.ContractAddress != nil {
		contractAddress = strings.TrimSpace(*mapping.ContractAddress)
	}
	if adjType == vaultAdjustmentTypeDecrease {
		bal, bErr := l.svcCtx.VaultBalanceRepo.FindByNetworkCurrencyContract(l.ctx, network.ID, assetCode, contractAddress)
		if bErr != nil || bal == nil {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INSUFFICIENT_BALANCE", "insufficient balance", nil)
		}

		lockedRaw, lockErr := l.sumPendingDecreaseRaw(network.ID, assetCode)
		if lockErr != nil {
			l.Logger.Errorf("sum pending decrease failed: %v", lockErr)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}
		available := bal.BalanceRaw - lockedRaw
		if available < amountRes.Raw {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INSUFFICIENT_BALANCE", "decrease amount exceeds available balance", nil)
		}
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	var created *model.VaultAdjustmentModel
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		adjRepo := l.svcCtx.VaultAdjustmentRepo.WithTx(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		m := &model.VaultAdjustmentModel{
			NetworkID:      network.ID,
			ChainID:        network.ChainID,
			Network:        network.Network,
			Currency:       assetCode,
			AdjustmentType: adjType,
			Amount:         amountRes.AmountStr,
			AmountRaw:      amountRes.Raw,
			AmountUSD:      "0",
			AmountUSDRaw:   0,
			Reason:         reason,
			Source:         source,
			SourceAddress:  sourceAddrPtr,
			TxHash:         txHashPtr,
			Attachments:    attachmentsBytes,
			Status:         vaultAdjustmentStatusPending,
			SubmittedBy:    current.ID,
			CreatedAt:      &now,
			UpdatedAt:      &now,
		}
		if err := adjRepo.Create(l.ctx, m); err != nil {
			return err
		}
		created = m

		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"chain_id":         network.ChainID,
			"network":          network.Network,
			"currency":         assetCode,
			"adjustment_type":  adjType,
			"amount":           amountRes.AmountStr,
			"amount_raw":       amountRes.Raw,
			"source":           source,
			"source_address":   maskAddress(sourceAddr),
			"tx_hash":          maskAddress(txHash),
			"submitted_by":     current.ID,
			"submitted_reason": reason,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "vault.adjustment.create",
			TargetType:  "vault_adjustment",
			TargetID:    fmt.Sprintf("%d", m.ID),
			Description: "创建 Vault 调整申请",
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		l.Logger.Errorf("create vault adjustment failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "create adjustment failed", nil)
	}

	return &pb.CreateVaultAdjustmentResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "ADJUSTMENT_SUBMITTED"),
		Data: &pb.CreateVaultAdjustmentData{
			Adjustment: toPBVaultAdjustmentItem(created),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

type vaultAttachmentDTO struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func marshalVaultAttachments(items []*pb.VaultAttachment) ([]byte, error) {
	if len(items) == 0 {
		return nil, nil
	}
	out := make([]vaultAttachmentDTO, 0, len(items))
	for _, it := range items {
		if it == nil {
			continue
		}
		name := strings.TrimSpace(it.Name)
		url := strings.TrimSpace(it.Url)
		if name == "" && url == "" {
			continue
		}
		if len(name) > 255 || len(url) > 2048 {
			return nil, fmt.Errorf("attachment too long")
		}
		out = append(out, vaultAttachmentDTO{Name: name, URL: url})
	}
	if len(out) == 0 {
		return nil, nil
	}
	return json.Marshal(out)
}

func (l *CreateVaultAdjustmentLogic) sumPendingDecreaseRaw(networkID int64, currency string) (int64, error) {
	type row struct {
		SumRaw int64 `gorm:"column:sum_raw"`
	}
	currency = strings.TrimSpace(currency)
	if networkID <= 0 || currency == "" {
		return 0, nil
	}
	var r row
	err := l.svcCtx.DB.WithContext(l.ctx).Model(&model.VaultAdjustmentModel{}).
		Select("COALESCE(SUM(amount_raw),0) as sum_raw").
		Where(
			"network_id = ? AND currency = ? AND adjustment_type = ? AND status IN ? AND deleted_at IS NULL",
			networkID,
			currency,
			vaultAdjustmentTypeDecrease,
			[]string{vaultAdjustmentStatusPending, vaultAdjustmentStatusApproved, vaultAdjustmentStatusProcessing},
		).
		Scan(&r).Error
	return r.SumRaw, err
}
