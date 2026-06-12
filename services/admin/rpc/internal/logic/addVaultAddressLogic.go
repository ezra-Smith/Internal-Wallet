package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type AddVaultAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddVaultAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddVaultAddressLogic {
	return &AddVaultAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== Vault Address Management ====================
func (l *AddVaultAddressLogic) AddVaultAddress(in *pb.AddVaultAddressRequest) (*pb.AddVaultAddressResponse, error) {
	// 1. 参数验证
	if in.NetworkId <= 0 {
		return &pb.AddVaultAddressResponse{
			Success:   false,
			Message:   "Invalid network_id",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, nil
	}

	address := strings.TrimSpace(in.Address)
	if address == "" {
		return &pb.AddVaultAddressResponse{
			Success:   false,
			Message:   "Address is required",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, nil
	}

	addressType := strings.TrimSpace(in.AddressType)
	if !validateVaultAddressType(addressType) {
		return &pb.AddVaultAddressResponse{
			Success:   false,
			Message:   fmt.Sprintf("Invalid address_type, must be one of: active, hot, cold, deposit"),
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, nil
	}

	// 2. 检查网络是否存在
	network, err := l.svcCtx.VaultNetworkRepo.FindByID(l.ctx, in.NetworkId)
	if err != nil || network == nil {
		return &pb.AddVaultAddressResponse{
			Success:   false,
			Message:   "Network not found",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, nil
	}

	// 3. 检查地址是否已存在
	existing, err := l.svcCtx.VaultAddressRepo.FindByAddress(l.ctx, in.NetworkId, address)
	if err != nil {
		logx.Errorf("Failed to check existing address: %v", err)
		return &pb.AddVaultAddressResponse{
			Success:   false,
			Message:   "Internal server error",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, err
	}
	if existing != nil {
		return &pb.AddVaultAddressResponse{
			Success:   false,
			Message:   "Address already exists for this network",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, nil
	}

	// 4. 获取创建人信息 (从 context 中获取 admin_id)
	adminID, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || adminID == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}
	// 5. 创建地址记录
	now := time.Now()
	var label *string
	if strings.TrimSpace(in.Label) != "" {
		l := strings.TrimSpace(in.Label)
		label = &l
	}

	addrModel := &model.VaultAddressModel{
		NetworkID:      in.NetworkId,
		Address:        address,
		AddressType:    addressType,
		Label:          label,
		Status:         vaultAddressStatusInactive, // 默认不启用
		IsActiveWallet: 0,
		CreatedBy:      adminID.ID,
		CreatedAt:      &now,
		UpdatedAt:      &now,
	}

	if err := l.svcCtx.VaultAddressRepo.Create(l.ctx, addrModel); err != nil {
		logx.Errorf("Failed to create vault address: %v", err)
		return &pb.AddVaultAddressResponse{
			Success:   false,
			Message:   "Failed to create address",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, err
	}

	// 6. 异步同步链上余额（参考 getWeb3AssetOverviewLogic.go 的实现）
	// 导入公司钱包地址后，自动从链上查询所有资产余额并写入 vault_address_balances
	go func() {
		syncCtx, syncCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer syncCancel()

		if err := SyncVaultAddressBalancesHelper(syncCtx, l.svcCtx, l.Logger, addrModel); err != nil {
			logx.Errorf("Failed to sync vault address balances for address_id=%d: %v", addrModel.ID, err)
		} else {
			logx.Infof("Successfully triggered balance sync for vault address_id=%d", addrModel.ID)
		}
	}()

	// 7. 返回成功
	return &pb.AddVaultAddressResponse{
		Success: true,
		Message: "Address added successfully, balance sync started",
		Data: &pb.AddVaultAddressData{
			Id:             addrModel.ID,
			NetworkId:      addrModel.NetworkID,
			Address:        addrModel.Address,
			AddressType:    addrModel.AddressType,
			Label:          utils.StringPtrToString(addrModel.Label),
			Status:         addrModel.Status,
			IsActiveWallet: addrModel.IsActiveWallet == 1,
			CreatedAt:      addrModel.CreatedAt.Format(time.RFC3339),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}
