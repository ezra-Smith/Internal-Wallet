package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/repository"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetDepositAddressBookDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDepositAddressBookDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDepositAddressBookDetailLogic {
	return &GetDepositAddressBookDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetDepositAddressBookDetailLogic) GetDepositAddressBookDetail(in *pb.GetDepositAddressBookDetailReq) (*pb.GetDepositAddressBookDetailResp, error) {
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

	if l.svcCtx.DepositAddressBookRepository == nil {
		return nil, errx.ServiceNotAvailable("db")
	}

	addr, err := l.svcCtx.DepositAddressBookRepository.FindActiveByID(l.ctx, uid, id)
	if err != nil || addr == nil {
		return nil, errx.NotFound("deposit address not found")
	}

	chainCode := strings.ToUpper(strings.TrimSpace(addr.ChainCode))
	assetCode := ""
	if in != nil {
		assetCode = strings.ToUpper(strings.TrimSpace(in.AssetCode))
	}

	// Chain info
	chainName := MapChainDisplayName(chainCode)
	iconURL := ""
	if l.svcCtx.ChainRepository != nil {
		if info, err := l.svcCtx.ChainRepository.FindByName(l.ctx, chainCode); err == nil && info != nil {
			if strings.TrimSpace(info.Network) != "" {
				chainName = strings.TrimSpace(info.Network)
			} else if strings.TrimSpace(info.Name) != "" {
				chainName = strings.TrimSpace(info.Name)
			}
			iconURL = strings.TrimSpace(info.IconUrl)
		}
	}

	// Stats (optional)
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

	label := ""
	if addr.Label != nil {
		label = strings.TrimSpace(*addr.Label)
	}
	remark := ""
	if addr.Remark != nil {
		remark = strings.TrimSpace(*addr.Remark)
	}

	totalReceived := ""
	receivedCount := int32(0)
	if assetCode != "" {
		totalReceived = "0 " + assetCode
		key := depositAddressStatKey(chainCode, addr.Address)
		if stat, ok := statsMap[key]; ok {
			amt := strings.TrimSpace(stat.TotalAmount)
			if amt == "" {
				amt = "0"
			}
			totalReceived = amt + " " + assetCode
			receivedCount = int32(stat.Count)
		}
	}

	return &pb.GetDepositAddressBookDetailResp{
		Success: true,
		Message: "ok",
		Item: &pb.DepositAddressBookItem{
			Id:               strconv.FormatInt(addr.ID, 10),
			Chain:            chainCode,
			ChainName:        chainName,
			IconUrl:          iconURL,
			Address:          strings.TrimSpace(addr.Address),
			Label:            label,
			Remark:           remark,
			IsDefault:        addr.IsDefault,
			DerivationChange: int32(addr.DerivationChange),
			CreatedAt:        formatTimePtr(addr.CreatedAt),
			LastUsedAt:       formatTimePtr(addr.LastUsedAt),
			TotalReceived:    totalReceived,
			ReceivedCount:    receivedCount,
		},
	}, nil
}
