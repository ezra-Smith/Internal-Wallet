package logic

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type GetCurrencyConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCurrencyConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCurrencyConfigLogic {
	return &GetCurrencyConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetCurrencyConfigLogic) GetCurrencyConfig(in *pb.GetCurrencyConfigRequest) (*pb.GetCurrencyConfigResponse, error) {
	if in == nil || strings.TrimSpace(in.AssetCode) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{"asset_code": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.AccountingRpc == nil || l.svcCtx.ChainRepo == nil || l.svcCtx.CurrencyChainSettingsRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	// 注意：此方法允许内部服务调用（business-rpc），认证已在拦截器中处理
	// 如果是内部服务调用，拦截器会直接放行；如果是管理员调用，拦截器会验证 token

	assetCode := normalizeCode(in.AssetCode)
	accResp, callErr := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: assetCode})
	if callErr != nil {
		l.Logger.Errorf("call accounting GetAsset failed: %v", callErr)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting service error", nil)
	}
	if accResp == nil || !accResp.Success || accResp.Item == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "asset not found", map[string]string{"asset_code": "not found"})
	}
	asset := accResp.Item

	// Settings (optional)
	var settingsPB *pb.CurrencyFeatureSettings
	var settingsModel *model.CurrencySettingsModel
	if l.svcCtx.CurrencySettingsRepo != nil {
		if s, sErr := l.svcCtx.CurrencySettingsRepo.GetByAssetCode(l.ctx, assetCode); sErr == nil {
			settingsModel = s
		}
	}
	if settingsModel == nil {
		settingsPB = &pb.CurrencyFeatureSettings{
			Web2DepositEnabled:     false,
			Web2WithdrawEnabled:    false,
			Web2TransferEnabled:    false,
			Web3DepositEnabled:     true,
			Web3WithdrawEnabled:    true,
			Web3SwapEnabled:        enforceBaseCurrencySwapEnabled(assetCode, false),
			UseGlobalWithdrawFee:   true,
			UseGlobalWithdrawAudit: true,
			UseGlobalTransferAudit: true,
		}
	} else {
		settingsPB = &pb.CurrencyFeatureSettings{
			Web2DepositEnabled:     settingsModel.Web2DepositEnabled,
			Web2WithdrawEnabled:    settingsModel.Web2WithdrawEnabled,
			Web2TransferEnabled:    settingsModel.Web2TransferEnabled,
			Web3DepositEnabled:     settingsModel.Web3DepositEnabled,
			Web3WithdrawEnabled:    settingsModel.Web3WithdrawEnabled,
			Web3SwapEnabled:        enforceBaseCurrencySwapEnabled(assetCode, settingsModel.Web3SwapEnabled),
			UseGlobalWithdrawFee:   settingsModel.UseGlobalWithdrawFee,
			UseGlobalWithdrawAudit: settingsModel.UseGlobalWithdrawAudit,
			UseGlobalTransferAudit: settingsModel.UseGlobalTransferAudit,
		}
	}

	chainsAll, _ := l.svcCtx.ChainRepo.ListAll(l.ctx)
	chainNameByCode := map[string]string{}
	chainIconByCode := map[string]string{}
	for _, c := range chainsAll {
		if c == nil {
			continue
		}
		cc := normalizeCode(c.Name)
		chainNameByCode[cc] = strings.TrimSpace(c.Network)
		chainIconByCode[cc] = strings.TrimSpace(c.IconUrl)
	}

	chainSettings, err := l.svcCtx.CurrencyChainSettingsRepo.ListByAssetCode(l.ctx, assetCode)
	if err != nil {
		l.Logger.Errorf("list currency chains failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	// Backward-compatible default: if no explicit mapping, expose all enabled chains as selectable.
	if len(chainSettings) == 0 {
		for _, c := range chainsAll {
			if c == nil || c.IsDeleted() {
				continue
			}
			chainSettings = append(chainSettings, &model.CurrencyChainSettingsModel{
				AssetCode:       assetCode,
				ChainCode:       normalizeCode(c.Name),
				DepositEnabled:  true,
				WithdrawEnabled: true,
				// Backward-compatible default for existing rows (before column was introduced): show in Web3.
				Web3AssetDisplayEnabled: true,
				ConsolidationEnabled:    false,
				Status:                  1,
			})
		}
	}

	chainItemsPB := make([]*pb.CurrencyChainSettingItem, 0, len(chainSettings))
	chainCodes := make([]string, 0, len(chainSettings))
	enabledCount := int32(0)
	for _, cs := range chainSettings {
		if cs == nil {
			continue
		}
		cc := normalizeCode(cs.ChainCode)
		chainCodes = append(chainCodes, cc)
		if cs.Status == 1 {
			enabledCount++
		}
		chainItemsPB = append(chainItemsPB, &pb.CurrencyChainSettingItem{
			Id:                   cs.ID,
			ChainCode:            cc,
			ChainName:            chainNameByCode[cc],
			Status:               cs.Status,
			DepositEnabled:       cs.DepositEnabled,
			WithdrawEnabled:      cs.WithdrawEnabled,
			ConsolidationEnabled: cs.ConsolidationEnabled,
			ChainIconUrl:         chainIconByCode[cc],
			Web3AssetDisplayEnabled: func() *wrapperspb.BoolValue {
				return wrapperspb.Bool(cs.Web3AssetDisplayEnabled)
			}(),
			ContractAddress: func() string {
				if cs.ContractAddress == nil {
					return ""
				}
				return strings.TrimSpace(*cs.ContractAddress)
			}(),
			TokenDecimals: func() *wrapperspb.Int32Value {
				if cs.TokenDecimals == nil {
					return nil
				}
				return wrapperspb.Int32(*cs.TokenDecimals)
			}(),
			MinWithdrawAmount: func() string {
				if cs.MinWithdrawAmount == nil {
					return ""
				}
				return strings.TrimSpace(*cs.MinWithdrawAmount)
			}(),
			MinDepositAmount: func() string {
				if cs.MinDepositAmount == nil {
					return ""
				}
				return strings.TrimSpace(*cs.MinDepositAmount)
			}(),
		})
	}

	feeRulesByChain := map[string][]*pb.CurrencyWithdrawFeeRuleItem{}
	if l.svcCtx.CurrencyWithdrawFeeRuleRepo != nil {
		var rows []*model.CurrencyWithdrawFeeRuleModel
		if err := l.svcCtx.DB.WithContext(l.ctx).
			Where("asset_code = ?", assetCode).
			Order("chain_code ASC, sort_order ASC, id ASC").
			Find(&rows).Error; err == nil {
			for _, r := range rows {
				if r == nil {
					continue
				}
				cc := normalizeCode(r.ChainCode)
				feeRulesByChain[cc] = append(feeRulesByChain[cc], &pb.CurrencyWithdrawFeeRuleItem{
					Id:       r.ID,
					RuleType: strings.ToLower(strings.TrimSpace(r.RuleType)),
					Value:    strings.TrimSpace(r.Value),
					MinFee: func() string {
						if r.MinFee == nil {
							return ""
						}
						return strings.TrimSpace(*r.MinFee)
					}(),
					MaxFee: func() string {
						if r.MaxFee == nil {
							return ""
						}
						return strings.TrimSpace(*r.MaxFee)
					}(),
					MinAmount: func() string {
						if r.MinAmount == nil {
							return ""
						}
						return strings.TrimSpace(*r.MinAmount)
					}(),
					MaxAmount: func() string {
						if r.MaxAmount == nil {
							return ""
						}
						return strings.TrimSpace(*r.MaxAmount)
					}(),
					Enabled:   r.Enabled,
					SortOrder: r.SortOrder,
				})
			}
		}
	}

	feeRulesPB := make([]*pb.CurrencyChainFeeRules, 0, len(chainCodes))
	for _, cc := range chainCodes {
		feeRulesPB = append(feeRulesPB, &pb.CurrencyChainFeeRules{
			ChainCode: cc,
			Rules:     feeRulesByChain[cc],
		})
	}

	auditRulesByChain := map[string][]*pb.CurrencyWithdrawAuditRuleItem{}
	if l.svcCtx.CurrencyWithdrawAuditRuleRepo != nil {
		var rows []*model.CurrencyWithdrawAuditRuleModel
		if err := l.svcCtx.DB.WithContext(l.ctx).
			Where("asset_code = ?", assetCode).
			Order("chain_code ASC, sort_order ASC, id ASC").
			Find(&rows).Error; err == nil {
			for _, r := range rows {
				if r == nil {
					continue
				}
				cc := normalizeCode(r.ChainCode)
				auditRulesByChain[cc] = append(auditRulesByChain[cc], &pb.CurrencyWithdrawAuditRuleItem{
					Id:        r.ID,
					MinAmount: strings.TrimSpace(r.MinAmount),
					MaxAmount: func() string {
						if r.MaxAmount == nil {
							return ""
						}
						return strings.TrimSpace(*r.MaxAmount)
					}(),
					Strategy:  strings.ToLower(strings.TrimSpace(r.Strategy)),
					Enabled:   r.Enabled,
					SortOrder: r.SortOrder,
				})
			}
		}
	}
	auditRulesPB := make([]*pb.CurrencyChainAuditRules, 0, len(chainCodes))
	for _, cc := range chainCodes {
		auditRulesPB = append(auditRulesPB, &pb.CurrencyChainAuditRules{
			ChainCode: cc,
			Rules:     auditRulesByChain[cc],
		})
	}

	name := strings.TrimSpace(asset.Name)
	if name == "" {
		name = assetCode
	}
	updatedTime := ""
	if settingsModel != nil {
		updatedTime = formatTime(settingsModel.UpdatedAt)
	}

	return &pb.GetCurrencyConfigResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%s)", assetCode),
		Data: &pb.GetCurrencyConfigData{
			Currency: &pb.CurrencyItem{
				AssetCode:    assetCode,
				AssetName:    name,
				Status:       asset.Status,
				Settings:     settingsPB,
				ChainTotal:   int32(len(chainItemsPB)),
				ChainEnabled: enabledCount,
				UpdatedAt:    updatedTime,
				IconUrl:      strings.TrimSpace(asset.IconUrl),
				Precision:    asset.Precision,
			},
			Chains:     chainItemsPB,
			FeeRules:   feeRulesPB,
			AuditRules: auditRulesPB,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
