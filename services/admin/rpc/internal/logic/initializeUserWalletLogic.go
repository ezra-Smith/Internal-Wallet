package logic

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
)

type InitializeUserWalletLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInitializeUserWalletLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InitializeUserWalletLogic {
	return &InitializeUserWalletLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InitializeUserWalletLogic) InitializeUserWallet(in *pb.InitializeUserWalletRequest) (*pb.InitializeUserWalletResponse, error) {
	if in == nil || in.UserId <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"user_id": "required",
		})
	}
	if l.svcCtx.DB == nil ||
		l.svcCtx.UserRepo == nil ||
		l.svcCtx.ChainRepo == nil ||
		l.svcCtx.WalletDepositAddressRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	// Check if SignerRpc is available
	if l.svcCtx.SignerRpc == nil {
		return nil, errx.New(codes.Unavailable, 503, errx.CodeInternalError, "SIGNER_UNAVAILABLE", "signer service not available", nil)
	}

	// Validate user exists
	if _, err := l.svcCtx.UserRepo.FindByID(l.ctx, in.UserId); err != nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "USER_NOT_FOUND", "user not found", nil)
	}

	chainFilter := normalizeChainFilter(in.Chains)

	// 获取启用的链（来自 ChainModel）
	enabledChains, err := l.svcCtx.ChainRepo.ListEnabled(l.ctx)
	if err != nil {
		l.Logger.Errorf("Failed to list enabled chains: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "failed to list chains", nil)
	}

	// 构建链代码列表（一个链一个地址）
	chainCodes := make([]string, 0)
	for _, c := range enabledChains {
		if c == nil {
			continue
		}
		chainCode := normalizeCode(c.Name)
		if chainCode == "" {
			continue
		}

		// 应用链过滤器（如果指定）
		if len(chainFilter) > 0 {
			if _, ok := chainFilter[chainCode]; !ok {
				continue
			}
		}

		chainCodes = append(chainCodes, chainCode)
	}

	if len(chainCodes) == 0 {
		if len(chainFilter) > 0 {
			return nil, errx.New(codes.FailedPrecondition, 422, errx.CodeInvalidParam, "NO_MATCHING_CHAINS", "no enabled chains match specified filter", nil)
		}
		return nil, errx.New(codes.FailedPrecondition, 422, errx.CodeInvalidParam, "NO_CHAINS", "no enabled chains available", nil)
	}

	sort.Strings(chainCodes)

	var createdAddresses []*model.WalletDepositAddressModel
	var totalCreated int32 = 0
	var totalSkipped int32 = 0
	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	requester := strings.TrimSpace(in.Requester)
	if requester == "" {
		requester = "unknown"
	}

	signerResp, err := l.svcCtx.SignerRpc.GenerateUserDepositAddresses(l.ctx, &pb.GenerateUserDepositAddressesRequest{
		SeedId:    defaultHotWalletSeedID,
		UserId:    in.UserId,
		Chains:    chainCodes,
		Requester: requester,
	})
	if err != nil {
		l.Logger.Errorf("Failed to generate user deposit addresses: user_id=%d, chains=%v, err=%v", in.UserId, chainCodes, err)
	} else if signerResp.Code != 200 {
		l.Logger.Errorf("Signer returned error: user_id=%d, chains=%v, code=%d, message=%s", in.UserId, chainCodes, signerResp.Code, signerResp.Message)
	}

	chainToAddress := make(map[string]string)
	if signerResp != nil {
		for _, a := range signerResp.Addresses {
			if a == nil {
				continue
			}
			chainCode := normalizeCode(a.Chain)
			address := strings.TrimSpace(a.Address)
			if chainCode == "" || address == "" {
				continue
			}
			chainToAddress[chainCode] = address
		}
	}

	for _, chainCode := range chainCodes {
		address, ok := chainToAddress[chainCode]
		if !ok || address == "" {
			l.Logger.Errorf("No address generated for chain %s, skip creating deposit address", chainCode)
			continue
		}

		// Check if address already exists
		if existing, err := l.svcCtx.WalletDepositAddressRepo.FindActiveByUserChain(l.ctx, in.UserId, chainCode); err == nil && existing != nil {
			l.Logger.Infof("Address already exists for user_id=%d, chain_code=%s, skipping", in.UserId, chainCode)
			totalSkipped++
			createdAddresses = append(createdAddresses, existing)
			continue
		}

		var created *model.WalletDepositAddressModel
		if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
			addrRepo := l.svcCtx.WalletDepositAddressRepo.WithTx(tx)
			auditRepo := repository.NewAdminAuditLogRepository(tx)

			m := &model.WalletDepositAddressModel{
				UserID:    in.UserId,
				ChainCode: chainCode,
				Address:   address,
				Status:    "active",
				Memo:      nil,
				IsDefault: true,
				// For user deposit addresses, derivation_change is always 0 for the initial default address.
				DerivationChange: 0,
				CreatedAt:        &now,
				UpdatedAt:        &now,
			}
			if err := addrRepo.Create(l.ctx, m); err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
					totalSkipped++
					return nil
				}
				return err
			}
			created = m
			totalCreated++

			detailsBytes, _ := json.Marshal(map[string]interface{}{
				"user_id":    in.UserId,
				"chain_code": chainCode,
				"address":    maskAddress(address),
				"requester":  requester,
			})
			_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
				AdminID:     0, // system action
				Action:      "wallet.user_wallet.initialize",
				TargetType:  "wallet_deposit_address",
				TargetID:    maskAddress(address),
				Description: "初始化用户钱包: " + maskAddress(address),
				Details:     detailsBytes,
				IP:          ip,
				UserAgent:   ua,
			})
			return nil
		}); err != nil {
			l.Logger.Errorf("Failed to create deposit address: user_id=%d, chain_code=%s, err=%v", in.UserId, chainCode, err)
			continue
		}

		if created != nil {
			createdAddresses = append(createdAddresses, created)
		}
	}

	// Convert to protobuf response
	pbAddresses := make([]*pb.WalletDepositAddressItem, 0, len(createdAddresses))
	for _, addr := range createdAddresses {
		pbAddresses = append(pbAddresses, toPBDepositAddressItem(addr))
	}

	return &pb.InitializeUserWalletResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "USER_WALLET_INITIALIZED"),
		Data: &pb.InitializeUserWalletData{
			TotalCreated: totalCreated,
			TotalSkipped: totalSkipped,
			Addresses:    pbAddresses,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

const defaultHotWalletSeedID = "hot_wallet_main"

func normalizeChainFilter(chains []string) map[string]struct{} {
	if len(chains) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(chains))
	for _, ch := range chains {
		ch = strings.TrimSpace(ch)
		if ch == "" {
			continue
		}
		code := normalizeCode(chainTypeToChainCode(ch))
		if code == "" {
			continue
		}
		out[code] = struct{}{}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
