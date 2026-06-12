package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/repository"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListDepositAddressBookLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListDepositAddressBookLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDepositAddressBookLogic {
	return &ListDepositAddressBookLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 资产-充值地址簿（同链多地址）
func (l *ListDepositAddressBookLogic) ListDepositAddressBook(in *pb.ListDepositAddressBookReq) (*pb.ListDepositAddressBookResp, error) {
	uidStr := middleware.GetUserID(l.ctx)
	if strings.TrimSpace(uidStr) == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, _ := strconv.ParseInt(uidStr, 10, 64)

	if l.svcCtx.DepositAddressBookRepository == nil {
		return nil, errx.ServiceNotAvailable("db")
	}

	chainCode := ""
	assetCode := ""
	page := int32(1)
	pageSize := int32(20)
	if in != nil {
		chainCode = strings.ToUpper(strings.TrimSpace(in.Chain))
		assetCode = strings.ToUpper(strings.TrimSpace(in.AssetCode))
		if in.Page > 0 {
			page = in.Page
		}
		if in.PageSize > 0 {
			pageSize = in.PageSize
		}
	}

	rows, total, err := l.svcCtx.DepositAddressBookRepository.ListActiveByUser(l.ctx, uid, chainCode, page, pageSize)
	if err != nil {
		l.Errorf("ListDepositAddressBook failed: %v", err)
		return nil, errx.Internal("list deposit addresses failed")
	}

	// Chain info cache
	chainInfoMap := make(map[string]*model.ChainModel)
	if l.svcCtx.ChainRepository != nil {
		for _, r := range rows {
			if r == nil {
				continue
			}
			ch := strings.ToUpper(strings.TrimSpace(r.ChainCode))
			if ch == "" {
				continue
			}
			if _, ok := chainInfoMap[ch]; ok {
				continue
			}
			if info, err := l.svcCtx.ChainRepository.FindByName(l.ctx, ch); err == nil && info != nil {
				chainInfoMap[ch] = info
			}
		}
	}

	// Receive stats cache (optional)
	statsMap := make(map[string]repository.DepositReceiveStat)
	if assetCode != "" && l.svcCtx.WalletDepositRepository != nil {
		stats, err := l.svcCtx.WalletDepositRepository.ListReceiveStatsByUser(l.ctx, uid, chainCode, assetCode, []string{"confirmed", "completed"})
		if err != nil {
			l.Errorf("ListReceiveStatsByUser failed: %v", err)
		} else {
			for _, s := range stats {
				key := depositAddressStatKey(s.ChainCode, s.DepositAddress)
				statsMap[key] = s
			}
		}
	}

	items := make([]*pb.DepositAddressBookItem, 0, len(rows))
	for _, r := range rows {
		if r == nil {
			continue
		}
		ch := strings.ToUpper(strings.TrimSpace(r.ChainCode))
		if ch == "" {
			continue
		}
		chainName := MapChainDisplayName(ch)
		iconURL := ""
		if info, ok := chainInfoMap[ch]; ok && info != nil {
			if strings.TrimSpace(info.Network) != "" {
				chainName = strings.TrimSpace(info.Network)
			} else if strings.TrimSpace(info.Name) != "" {
				chainName = strings.TrimSpace(info.Name)
			}
			iconURL = strings.TrimSpace(info.IconUrl)
		}

		label := ""
		if r.Label != nil {
			label = strings.TrimSpace(*r.Label)
		}
		remark := ""
		if r.Remark != nil {
			remark = strings.TrimSpace(*r.Remark)
		}

		totalReceived := ""
		receivedCount := int32(0)
		if assetCode != "" {
			totalReceived = "0 " + assetCode
			key := depositAddressStatKey(ch, r.Address)
			if stat, ok := statsMap[key]; ok {
				amt := strings.TrimSpace(stat.TotalAmount)
				if amt == "" {
					amt = "0"
				}
				totalReceived = amt + " " + assetCode
				receivedCount = int32(stat.Count)
			}
		}

		items = append(items, &pb.DepositAddressBookItem{
			Id:               strconv.FormatInt(r.ID, 10),
			Chain:            ch,
			ChainName:        chainName,
			IconUrl:          iconURL,
			Address:          strings.TrimSpace(r.Address),
			Label:            label,
			Remark:           remark,
			IsDefault:        r.IsDefault,
			DerivationChange: int32(r.DerivationChange),
			CreatedAt:        formatTimePtr(r.CreatedAt),
			LastUsedAt:       formatTimePtr(r.LastUsedAt),
			TotalReceived:    totalReceived,
			ReceivedCount:    receivedCount,
		})
	}

	return &pb.ListDepositAddressBookResp{
		Success: true,
		Total:   int32(total),
		Items:   items,
	}, nil
}
