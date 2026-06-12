package logic

import (
	"context"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetWithdrawDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWithdrawDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWithdrawDetailLogic {
	return &GetWithdrawDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetWithdrawDetailLogic) GetWithdrawDetail(in *pb.GetWithdrawDetailReq) (*pb.GetWithdrawDetailResp, error) {
	if in == nil || in.Id == "" {
		return nil, errx.InvalidParam("invalid params")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, _ := strconv.ParseInt(uidStr, 10, 64)
	id, err := strconv.ParseInt(in.Id, 10, 64)
	if err != nil {
		return nil, errx.InvalidParam("invalid id")
	}

	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.ServiceNotAvailable("accounting rpc")
	}
	accResp, err := l.svcCtx.AccountingRpc.GetUserTransactionRecordDetail(l.ctx, &pb.GetUserTransactionRecordDetailRequest{Id: id})
	if err != nil || accResp == nil || !accResp.Success || accResp.Item == nil {
		return nil, errx.WithdrawNotFound()
	}

	item := accResp.Item
	if item.UserId != uid || item.TxType != 2 {
		return nil, errx.WithdrawNotFound()
	}

	asset := strings.ToUpper(strings.TrimSpace(item.AssetCode))
	chainCode := strings.ToUpper(strings.TrimSpace(item.ChainCode))
	txHash := strings.TrimSpace(item.TxHash)

	// Get chain info for display name and explorer URL
	chain := GetChainInfo(l.ctx, l.svcCtx, chainCode)
	chainName := MapChainDisplayName(chainCode)
	if chain != nil && strings.TrimSpace(chain.Network) != "" {
		chainName = strings.TrimSpace(chain.Network)
	}

	// Build explorer URL
	explorerURL := ""
	if txHash != "" {
		if chain != nil {
			explorerURL = BuildExplorerURLFromChain(chain, txHash)
		}
		if explorerURL == "" {
			explorerURL = BuildExplorerURL(chainCode, txHash)
		}
	}

	// Get from/to address from withdraw order for more accuracy
	fromAddress := strings.TrimSpace(item.FromAddress)
	toAddress := strings.TrimSpace(item.ToAddress)
	submittedAt := ""
	processedAt := ""
	rejectedReason := ""

	if item.Timestamp > 0 {
		submittedAt = time.Unix(item.Timestamp, 0).Format("2006/01/02 15:04:05")
	}

	if l.svcCtx.CurrencyWithdrawOrderRepository != nil {
		if order, err := l.svcCtx.CurrencyWithdrawOrderRepository.FindByUserIDAndID(l.ctx, uid, id); err == nil && order != nil {
			if fromAddress == "" {
				fromAddress = strings.TrimSpace(order.FromAddress)
			}
			if strings.TrimSpace(order.ToAddress) != "" {
				toAddress = strings.TrimSpace(order.ToAddress)
			}
			if order.TxHash != nil && strings.TrimSpace(*order.TxHash) != "" {
				txHash = strings.TrimSpace(*order.TxHash)
				// Rebuild explorer URL with actual tx hash
				if chain != nil {
					explorerURL = BuildExplorerURLFromChain(chain, txHash)
				}
				if explorerURL == "" {
					explorerURL = BuildExplorerURL(chainCode, txHash)
				}
			}
			submittedAt = order.CreatedAt.Format("2006/01/02 15:04:05")
			if order.TransferredAt != nil {
				processedAt = order.TransferredAt.Format("2006/01/02 15:04:05")
			}
			if order.ErrorMessage != nil && strings.TrimSpace(*order.ErrorMessage) != "" {
				rejectedReason = strings.TrimSpace(*order.ErrorMessage)
			}
		}
	}

	// Address labels
	fromLabel := "我的钱包"
	toLabel := ""

	// Format created_at
	createdAt := ""
	if item.Timestamp > 0 {
		createdAt = time.Unix(item.Timestamp, 0).Format("2006/01/02 15:04:05")
	}

	// Confirmations
	requiredConfs := GetRequiredConfirmations(chainCode)
	confirmations := int32(0)
	status := strings.ToLower(strings.TrimSpace(item.Status))
	if status == "success" || status == "completed" {
		confirmations = requiredConfs
	}

	// Fee info
	fee := strings.TrimSpace(item.FeeDecimal)
	feeAsset := asset

	// Get asset icon URL
	iconURL := GetAssetIconURL(l.ctx, l.svcCtx, asset)

	return &pb.GetWithdrawDetailResp{
		Success:               true,
		Id:                    strconv.FormatInt(item.Id, 10),
		Asset:                 asset,
		Amount:                strings.TrimSpace(item.AmountDecimal),
		AmountDisplay:         FormatAmountDisplay(item.AmountDecimal, asset, false),
		Status:                item.Status,
		StatusText:            MapStatusText(item.Status),
		Chain:                 chainCode,
		ChainName:             chainName,
		FromAddress:           fromAddress,
		ToAddress:             toAddress,
		FromLabel:             fromLabel,
		ToLabel:               toLabel,
		TxHash:                txHash,
		ExplorerUrl:           explorerURL,
		Fee:                   fee,
		FeeAsset:              feeAsset,
		Confirmations:         confirmations,
		RequiredConfirmations: requiredConfs,
		CreatedAt:             createdAt,
		SubmittedAt:           submittedAt,
		ProcessedAt:           processedAt,
		RejectedReason:        rejectedReason,
		Type:                  "withdraw",
		TypeText:              "Withdraw",
		Method:                "on_chain",
		MethodText:            "On-chain",
		IconUrl:               iconURL,
	}, nil
}
