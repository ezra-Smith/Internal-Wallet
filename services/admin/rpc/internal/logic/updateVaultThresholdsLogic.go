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
	"gorm.io/gorm"
)

type UpdateVaultThresholdsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateVaultThresholdsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateVaultThresholdsLogic {
	return &UpdateVaultThresholdsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateVaultThresholdsLogic) UpdateVaultThresholds(in *pb.UpdateVaultThresholdsRequest) (*pb.UpdateVaultThresholdsResponse, error) {
	if in == nil {
		in = &pb.UpdateVaultThresholdsRequest{}
	}
	if l.svcCtx.DB == nil ||
		l.svcCtx.AccountingRpc == nil ||
		l.svcCtx.CurrencyChainSettingsRepo == nil ||
		l.svcCtx.VaultNetworkRepo == nil ||
		l.svcCtx.VaultThresholdRepo == nil {
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
	supported := false
	for _, s := range settings {
		if s == nil || s.IsDeleted() {
			continue
		}
		if normalizeCode(s.ChainCode) == chainCode && s.Status == 1 {
			supported = true
			break
		}
	}
	if !supported {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_CURRENCY", "currency not supported on this network", map[string]string{"currency": "chain_mismatch"})
	}

	if in.Thresholds == nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_THRESHOLDS", "thresholds required", map[string]string{"thresholds": "required"})
	}

	lowRes, err := parseAmountToRaw(in.Thresholds.Low, asset.Precision, true)
	if err != nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_THRESHOLD_LOW", "invalid thresholds.low", map[string]string{"thresholds.low": err.Error()})
	}
	criticalRes, err := parseAmountToRaw(in.Thresholds.Critical, asset.Precision, true)
	if err != nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_THRESHOLD_CRITICAL", "invalid thresholds.critical", map[string]string{"thresholds.critical": err.Error()})
	}
	if lowRes.Raw < criticalRes.Raw {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_THRESHOLDS", "low must be >= critical", map[string]string{"thresholds": "low<critical"})
	}

	notif := vaultThresholdNotifications{}
	if in.Notifications != nil {
		notif.EmailEnabled = in.Notifications.EmailEnabled
		notif.WebhookEnabled = in.Notifications.WebhookEnabled
		notif.WebhookURL = strings.TrimSpace(in.Notifications.WebhookUrl)
		for _, r := range in.Notifications.EmailRecipients {
			r = strings.TrimSpace(r)
			if r == "" {
				continue
			}
			notif.EmailRecipients = append(notif.EmailRecipients, r)
		}
	}
	notifBytes, _ := json.Marshal(notif)

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		thRepo := l.svcCtx.VaultThresholdRepo.WithTx(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		m := &model.VaultThresholdModel{
			NetworkID:            network.ID,
			Currency:             assetCode,
			ThresholdLow:         lowRes.AmountStr,
			ThresholdLowRaw:      lowRes.Raw,
			ThresholdCritical:    criticalRes.AmountStr,
			ThresholdCriticalRaw: criticalRes.Raw,
			Notifications:        notifBytes,
			UpdatedBy:            current.ID,
			CreatedAt:            &now,
			UpdatedAt:            &now,
		}
		if err := thRepo.Upsert(l.ctx, m); err != nil {
			return err
		}

		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"chain_id":   network.ChainID,
			"network":    network.Network,
			"currency":   assetCode,
			"low":        lowRes.AmountStr,
			"critical":   criticalRes.AmountStr,
			"updated_by": current.ID,
			"notify":     notif,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "vault.threshold.update",
			TargetType:  "vault_threshold",
			TargetID:    network.Network + ":" + assetCode,
			Description: "更新 Vault 阈值配置",
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		l.Logger.Errorf("update vault thresholds failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "update thresholds failed", nil)
	}

	return &pb.UpdateVaultThresholdsResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "THRESHOLDS_UPDATED"),
		Data: &pb.UpdateVaultThresholdsData{
			Network:  network.Network,
			ChainId:  network.ChainID,
			Currency: assetCode,
			Thresholds: &pb.VaultThresholds{
				Low:      lowRes.AmountStr,
				Critical: criticalRes.AmountStr,
			},
			Notifications: &pb.VaultThresholdNotifications{
				EmailEnabled:    notif.EmailEnabled,
				EmailRecipients: notif.EmailRecipients,
				WebhookEnabled:  notif.WebhookEnabled,
				WebhookUrl:      notif.WebhookURL,
			},
			UpdatedAt: formatTime(now),
			UpdatedBy: current.ID,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
