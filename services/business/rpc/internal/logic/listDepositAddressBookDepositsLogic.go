package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListDepositAddressBookDepositsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListDepositAddressBookDepositsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDepositAddressBookDepositsLogic {
	return &ListDepositAddressBookDepositsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListDepositAddressBookDepositsLogic) ListDepositAddressBookDeposits(in *pb.ListDepositAddressBookDepositsReq) (*pb.ListDepositAddressBookDepositsResp, error) {
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, errx.InvalidParam("invalid params")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if strings.TrimSpace(uidStr) == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, _ := strconv.ParseInt(uidStr, 10, 64)

	id, err := strconv.ParseInt(strings.TrimSpace(in.Id), 10, 64)
	if err != nil || id <= 0 {
		return nil, errx.InvalidParam("invalid params")
	}

	page := int32(1)
	pageSize := int32(20)
	assetCode := ""
	if in != nil {
		if in.Page > 0 {
			page = in.Page
		}
		if in.PageSize > 0 {
			pageSize = in.PageSize
		}
		assetCode = strings.ToUpper(strings.TrimSpace(in.Asset))
	}

	if l.svcCtx.DepositAddressBookRepository == nil || l.svcCtx.WalletDepositRepository == nil {
		return nil, errx.ServiceNotAvailable("db")
	}

	addr, err := l.svcCtx.DepositAddressBookRepository.FindActiveByID(l.ctx, uid, id)
	if err != nil || addr == nil {
		return nil, errx.NotFound("deposit address not found")
	}

	chainCode := strings.ToUpper(strings.TrimSpace(addr.ChainCode))
	depositAddress := strings.TrimSpace(addr.Address)

	deposits, total, err := l.svcCtx.WalletDepositRepository.ListByUserChainDepositAddress(l.ctx, uid, chainCode, depositAddress, assetCode, page, pageSize)
	if err != nil {
		l.Errorf("ListByUserChainDepositAddress failed: %v", err)
		return nil, errx.Internal("list deposits failed")
	}

	// Pre-load chain info
	var chainInfo *model.ChainModel
	chainName := MapChainDisplayName(chainCode)
	explorerBase := ""
	if l.svcCtx.ChainRepository != nil {
		if info, err := l.svcCtx.ChainRepository.FindByName(l.ctx, chainCode); err == nil && info != nil {
			chainInfo = info
			if strings.TrimSpace(info.Network) != "" {
				chainName = strings.TrimSpace(info.Network)
			}
			explorerBase = strings.TrimSpace(info.ExplorerUrl)
		}
	}

	// Pre-load asset metadata (icon URLs) from Accounting service (best-effort).
	assetIconMap := make(map[string]string)
	if l.svcCtx.AccountingRpc != nil {
		listResp, err := l.svcCtx.AccountingRpc.ListAssets(l.ctx, &pb.ListAssetsRequest{
			Page:     1,
			PageSize: 200,
			Status:   1,
		})
		if err != nil {
			l.Errorf("ListAssets failed: %v", err)
		} else if listResp != nil {
			for _, a := range listResp.GetItems() {
				if a == nil {
					continue
				}
				code := strings.ToUpper(strings.TrimSpace(a.Code))
				if code != "" {
					assetIconMap[code] = strings.TrimSpace(a.IconUrl)
				}
			}
		}
	}

	items := make([]*pb.DepositRecordItem, 0, len(deposits))
	requiredConfs := GetRequiredConfirmations(chainCode)
	for _, d := range deposits {
		if d == nil {
			continue
		}
		asset := strings.ToUpper(strings.TrimSpace(d.AssetCode))
		txHash := strings.TrimSpace(d.TransactionHash)

		explorerURL := ""
		if explorerBase != "" && txHash != "" {
			if strings.Contains(explorerBase, "%s") {
				explorerURL = strings.Replace(explorerBase, "%s", txHash, 1)
			} else {
				explorerURL = strings.TrimSuffix(explorerBase, "/") + "/tx/" + txHash
			}
		}
		if explorerURL == "" && txHash != "" {
			explorerURL = BuildExplorerURLFromChain(chainInfo, txHash)
		}

		createdAt := formatTimePtr(d.CreatedAt)
		status := strings.TrimSpace(d.Status)

		items = append(items, &pb.DepositRecordItem{
			Id:                    strconv.FormatInt(d.ID, 10),
			Asset:                 asset,
			Chain:                 chainCode,
			ChainName:             chainName,
			Amount:                strings.TrimSpace(d.Amount),
			AmountDisplay:         FormatAmountDisplay(d.Amount, asset, true),
			FromAddress:           "",
			ToAddress:             strings.TrimSpace(d.DepositAddress),
			TxHash:                txHash,
			ExplorerUrl:           explorerURL,
			Status:                status,
			StatusText:            MapStatusText(status),
			Confirmations:         d.Confirmations,
			RequiredConfirmations: requiredConfs,
			CreatedAt:             createdAt,
			ConfirmedAt:           "",
			IconUrl:               assetIconMap[asset],
		})
	}

	return &pb.ListDepositAddressBookDepositsResp{
		Success: true,
		Total:   int32(total),
		Items:   items,
	}, nil
}
