package logic

import (
	"context"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/common/security"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/repository"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type SetMyWithdrawAuditWhitelistBindingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetMyWithdrawAuditWhitelistBindingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetMyWithdrawAuditWhitelistBindingLogic {
	return &SetMyWithdrawAuditWhitelistBindingLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SetMyWithdrawAuditWhitelistBindingLogic) SetMyWithdrawAuditWhitelistBinding(in *pb.SetMyWithdrawAuditWhitelistBindingReq) (*pb.SetMyWithdrawAuditWhitelistBindingResp, error) {
	if in == nil || len(in.Items) == 0 {
		return nil, errx.InvalidParam("invalid params: items required")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	userID, _ := strconv.ParseInt(uidStr, 10, 64)
	if userID <= 0 {
		return nil, errx.Unauthorized("unauthorized")
	}

	if l.svcCtx == nil || l.svcCtx.DB == nil || l.svcCtx.UserWithdrawAuditWhitelistRuleRepository == nil || l.svcCtx.CurrencyChainSettingsRepository == nil || l.svcCtx.UserAccountRepository == nil {
		return nil, errx.Internal("db unavailable")
	}

	tradePassword := strings.TrimSpace(in.TradePassword)
	hasTrade := tradePassword != ""
	hasBio := in.Biometric != nil
	if hasTrade == hasBio {
		return nil, errx.InvalidParam("trade_password or biometric required")
	}
	if hasTrade {
		tradePasswordHash, hasTradePassword, err := l.svcCtx.UserAccountRepository.GetTradePasswordHash(l.ctx, userID)
		if err != nil {
			l.Logger.Errorf("Failed to get trade password hash: %v", err)
			return nil, errx.Internal("internal error")
		}
		if !hasTradePassword || strings.TrimSpace(tradePasswordHash) == "" {
			return nil, errx.TradePasswordNotSet()
		}
		if !security.VerifyPassword(tradePasswordHash, tradePassword) {
			return nil, errx.InvalidTradePassword()
		}
	} else {
		payloadHash, err := calcWithdrawAuditWhitelistBindingPayloadHash(in.Items)
		if err != nil {
			return nil, errx.InvalidParam("invalid params")
		}
		if err := verifyBiometricProof(l.ctx, l.svcCtx, userID, biometricSceneWithdrawAuditWhitelistBind, payloadHash, in.Biometric); err != nil {
			return nil, err
		}
	}

	// Prepare result items to return
	// Since we are processing a batch, we'll return the success status of the batch, or fail the entire batch.
	// For now, consistent with transactional logic, either all succeed or all fail.

	// Pre-validate all items
	type validatedItem struct {
		chainCode  string
		address    string
		walletName string
		walletIcon string
		enabled    bool
		assetCodes []string
	}
	validatedItems := make([]validatedItem, 0, len(in.Items))
	seenChainCodes := make(map[string]struct{}, len(in.Items))

	for _, item := range in.Items {
		chainCode := normalizeCode(item.ChainCode)
		if chainCode == "" {
			return nil, errx.InvalidParamWithFields("invalid params", map[string]string{"chain_code": "required"})
		}
		if _, exists := seenChainCodes[chainCode]; exists {
			return nil, errx.InvalidParamWithFields("invalid params", map[string]string{"chain_code": "duplicate"})
		}
		seenChainCodes[chainCode] = struct{}{}

		if !item.Enabled {
			// Unbinding: allow simpler payload
			validatedItems = append(validatedItems, validatedItem{
				chainCode: chainCode,
				enabled:   false,
			})
			continue
		}

		// Binding/Updating
		walletName, err := normalizeWithdrawAuditWhitelistWalletNameRequired(item.WalletName)
		if err != nil {
			code := "invalid"
			msg := strings.ToLower(err.Error())
			switch {
			case strings.Contains(msg, "required"):
				code = "required"
			case strings.Contains(msg, "too long"):
				code = "too_long"
			}
			return nil, errx.InvalidParamWithFields("invalid params", map[string]string{"wallet_name": code})
		}
		walletIcon, err := normalizeWithdrawAuditWhitelistWalletIcon(item.WalletIcon)
		if err != nil {
			code := "invalid"
			msg := strings.ToLower(err.Error())
			switch {
			case strings.Contains(msg, "too large"):
				code = "too_large"
			case strings.Contains(msg, "invalid"):
				code = "invalid"
			}
			return nil, errx.InvalidParamWithFields("invalid params", map[string]string{"wallet_icon": code})
		}

		address := normalizeWhitelistAddress(strings.TrimSpace(item.Address))
		if address == "" {
			return nil, errx.InvalidParamWithFields("invalid params", map[string]string{"address": "required"})
		}
		if !isValidWalletAddress(chainCode, address) {
			return nil, errx.InvalidParamWithFields("invalid params", map[string]string{"address": "invalid"})
		}

		assetCodes, err := l.svcCtx.CurrencyChainSettingsRepository.ListWithdrawEnabledAssetsByChain(l.ctx, chainCode)
		if err != nil {
			return nil, errx.DBError()
		}

		validatedItems = append(validatedItems, validatedItem{
			chainCode:  chainCode,
			address:    address,
			walletName: walletName,
			walletIcon: walletIcon,
			enabled:    true,
			assetCodes: assetCodes,
		})
	}

	if err := l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		ruleRepo := repository.NewUserWithdrawAuditWhitelistRuleRepository(tx)

		for _, item := range validatedItems {
			// Always soft delete existing rules for this user+chain+source first
			if err := ruleRepo.SoftDeleteByUserChainSource(l.ctx, userID, item.chainCode, "user"); err != nil {
				return err
			}

			if !item.enabled || len(item.assetCodes) == 0 {
				continue
			}

			rules := make([]*model.UserWithdrawAuditWhitelistRuleModel, 0, len(item.assetCodes))
			for _, assetCode := range item.assetCodes {
				assetCode = normalizeCode(assetCode)
				if assetCode == "" {
					continue
				}
				rules = append(rules, &model.UserWithdrawAuditWhitelistRuleModel{
					UserID:          userID,
					AssetCode:       assetCode,
					ChainCode:       item.chainCode,
					Address:         item.address,
					WalletName:      item.walletName,
					WalletIcon:      item.walletIcon,
					LimitUSDT:       "0",
					Enabled:         true,
					Reason:          "",
					OperatorAdminID: 0,
					Source:          "user",
					OperatorUserID:  userID,
				})
			}
			if len(rules) > 0 {
				if err := tx.WithContext(l.ctx).CreateInBatches(rules, 200).Error; err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return nil, errx.DBError()
	}

	rows, err := l.svcCtx.UserWithdrawAuditWhitelistRuleRepository.ListActiveByUserSource(l.ctx, userID, "user")
	if err != nil {
		return nil, errx.DBError()
	}
	items := make([]*pb.WithdrawAuditWhitelistBindingItem, 0, len(rows))
	for _, r := range rows {
		if r == nil {
			continue
		}
		src := strings.TrimSpace(r.Source)
		if src == "" {
			src = "admin"
		}
		items = append(items, &pb.WithdrawAuditWhitelistBindingItem{
			AssetCode:  strings.TrimSpace(r.AssetCode),
			ChainCode:  strings.TrimSpace(r.ChainCode),
			Address:    strings.TrimSpace(r.Address),
			WalletName: strings.TrimSpace(r.WalletName),
			WalletIcon: strings.TrimSpace(r.WalletIcon),
			Enabled:    r.Enabled,
			UpdatedAt:  r.UpdatedAt.Format(time.RFC3339),
			Source:     src,
		})
	}

	return &pb.SetMyWithdrawAuditWhitelistBindingResp{
		Success: true,
		Message: "ok",
		Items:   items,
	}, nil
}
