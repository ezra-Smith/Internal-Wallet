package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListInternalWalletAddressesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListInternalWalletAddressesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListInternalWalletAddressesLogic {
	return &ListInternalWalletAddressesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListInternalWalletAddressesLogic) ListInternalWalletAddresses(in *pb.ListInternalWalletAddressesReq) (*pb.ListInternalWalletAddressesResp, error) {
	uidStr := middleware.GetUserID(l.ctx)
	uid, _ := strconv.ParseInt(uidStr, 10, 64)

	repo := l.svcCtx.MemberInternalAddressRepository
	if repo == nil {
		return &pb.ListInternalWalletAddressesResp{Success: true, Items: []*pb.InternalWalletAddressItem{}}, nil
	}

	rows, err := repo.ListByUserID(l.ctx, uid)
	if err != nil {
		l.Logger.Errorf("ListByUserID failed: %v", err)
		return &pb.ListInternalWalletAddressesResp{Success: true, Items: []*pb.InternalWalletAddressItem{}}, nil
	}

	items := make([]*pb.InternalWalletAddressItem, 0, len(rows))

	for _, w := range rows {
		// 推断 lookup_type
		// 规则：如果 target_user_display 包含 @，则认为是邮箱；否则认为是 UID
		lookupType := "uid"

		if strings.Contains(w.TargetUserDisplay, "@") {
			// 包含 @，判断为邮箱
			lookupType = "email"
		}

		item := &pb.InternalWalletAddressItem{
			Id:                strconv.FormatInt(w.ID, 10),
			TargetUserId:      strconv.FormatInt(w.TargetUserID, 10),
			TargetUserDisplay: w.TargetUserDisplay,
			Label:             w.Label,
			Remark:            w.Remark,
			IsDefault:         w.IsDefault,
			CreatedAt:         w.CreatedAt.Unix(),
			LastUsedAt:        0, // 数据库中暂无此字段，设置为 0
			TransferCount:     0, // 数据库中暂无此字段，设置为 0
			LookupType:        lookupType,
		}

		items = append(items, item)
	}

	return &pb.ListInternalWalletAddressesResp{Success: true, Items: items}, nil
}
