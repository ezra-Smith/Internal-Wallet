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

type GetDepositDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDepositDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDepositDetailLogic {
	return &GetDepositDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetDepositDetailLogic) GetDepositDetail(in *pb.GetDepositDetailReq) (*pb.GetDepositDetailResp, error) {
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
		return nil, errx.DepositNotFound()
	}

	item := accResp.Item
	// Security: ensure the record belongs to the requesting user and is a deposit record
	if item.UserId != uid || item.TxType != 1 {
		return nil, errx.DepositNotFound()
	}

	asset := strings.ToUpper(strings.TrimSpace(item.AssetCode))
	chainCode := strings.ToUpper(strings.TrimSpace(item.ChainCode))
	txHash := strings.TrimSpace(item.TxHash)
	fromAddress := strings.TrimSpace(item.FromAddress)

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

	// Get user's deposit address (to_address)
	toAddress := strings.TrimSpace(item.ToAddress)
	if toAddress == "" {
		toAddress = GetUserDepositAddress(l.ctx, l.svcCtx, uid, chainCode)
	}

	// Address labels
	fromLabel := ""
	toLabel := "我的钱包"

	// Format timestamps
	createdAt := ""
	if item.Timestamp > 0 {
		createdAt = time.Unix(item.Timestamp, 0).Format("2006/01/02 15:04:05")
	}
	confirmedAt := "" // TODO: track confirmed_at in model

	// Confirmations
	requiredConfs := GetRequiredConfirmations(chainCode)
	confirmations := int32(0)
	status := strings.ToLower(strings.TrimSpace(item.Status))
	if status == "success" || status == "completed" {
		confirmations = requiredConfs
		if createdAt != "" && confirmedAt == "" {
			confirmedAt = createdAt // For completed deposits, confirmed_at = created_at if not tracked
		}
	}

	// Fee info
	fee := strings.TrimSpace(item.FeeDecimal)
	feeAsset := chainCode // Network fees are typically in chain's native token
	switch chainCode {
	case "TRON":
		feeAsset = "TRX"
	case "ETH":
		feeAsset = "ETH"
	case "BSC":
		feeAsset = "BNB"
	case "POLYGON":
		feeAsset = "MATIC"
	case "BTC":
		feeAsset = "BTC"
	}

	// Get asset icon URL
	iconURL := GetAssetIconURL(l.ctx, l.svcCtx, asset)

	return &pb.GetDepositDetailResp{
		Success:               true,
		Id:                    strconv.FormatInt(item.Id, 10),
		Asset:                 asset,
		Amount:                strings.TrimSpace(item.AmountDecimal),
		AmountDisplay:         FormatAmountDisplay(item.AmountDecimal, asset, true),
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
		ConfirmedAt:           confirmedAt,
		Type:                  "deposit",
		TypeText:              "Deposit",
		Method:                "on_chain",
		MethodText:            "On-chain",
		IconUrl:               iconURL,
	}, nil
}
