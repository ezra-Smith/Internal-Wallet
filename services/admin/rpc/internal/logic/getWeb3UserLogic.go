package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetWeb3UserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWeb3UserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWeb3UserLogic {
	return &GetWeb3UserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func parseTokenBalances(raw []byte) []*pb.Web3TokenBalance {
	if len(raw) == 0 {
		return nil
	}
	type row struct {
		Currency   string `json:"currency"`
		Balance    string `json:"balance"`
		BalanceUsd string `json:"balance_usd"`
	}
	var rows []row
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil
	}
	out := make([]*pb.Web3TokenBalance, 0, len(rows))
	for _, r := range rows {
		out = append(out, &pb.Web3TokenBalance{
			Currency:   strings.TrimSpace(r.Currency),
			Balance:    strings.TrimSpace(r.Balance),
			BalanceUsd: strings.TrimSpace(r.BalanceUsd),
		})
	}
	return out
}

func (l *GetWeb3UserLogic) GetWeb3User(in *pb.GetWeb3UserRequest) (*pb.GetWeb3UserResponse, error) {
	if in == nil || strings.TrimSpace(in.DeviceId) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "device_id required", map[string]string{"device_id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.Web3UserRepo == nil || l.svcCtx.Web3UserAddressRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	user, err := l.svcCtx.Web3UserRepo.FindByDeviceID(l.ctx, in.DeviceId)
	if err != nil || user == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "DEVICE_NOT_FOUND", "device not found", nil)
	}

	addrs, err := l.svcCtx.Web3UserAddressRepo.ListByUserID(l.ctx, user.ID)
	if err != nil {
		l.Logger.Errorf("list web3 addresses failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	walletAddrs := make([]*pb.Web3WalletAddress, 0, len(addrs))
	for _, a := range addrs {
		if a == nil {
			continue
		}
		walletAddrs = append(walletAddrs, toPBWeb3WalletAddress(a))
	}

	// Get transaction summary for this user
	txSummary, err := l.svcCtx.Web3TransactionRepo.GetUserTransactionSummary(l.ctx, user.ID)
	if err != nil {
		l.Logger.Errorf("get transaction summary failed: %v", err)
		// Don't fail the entire request if summary fails, just use empty values
		txSummary.TotalTransactions = 0
		txSummary.TotalDepositUSD = "0"
		txSummary.TotalWithdrawalUSD = "0"
		txSummary.TotalSwapUSD = "0"
	}

	d := &pb.Web3UserDetail{
		Id:       fmt.Sprintf("web3-%d", user.ID),
		DeviceId: user.DeviceID,
		DeviceInfo: &pb.Web3DeviceInfo{
			Platform:     user.Platform,
			OsVersion:    user.OsVersion,
			AppVersion:   user.AppVersion,
			DeviceModel:  user.DeviceModel,
			DeviceName:   user.DeviceName,
			PushToken:    derefString(user.PushToken),
			Locale:       derefString(user.Locale),
			LastActiveAt: formatTimePtr(user.LastActiveAt),
			CreatedAt:    formatTimePtr(user.CreatedAt),
		},
		SecurityInfo: &pb.Web3SecurityInfo{
			TwoFactorEnabled:   user.TwoFactorEnabled,
			TwoFactorType:      derefString(user.TwoFactorType),
			BiometricEnabled:   user.BiometricEnabled,
			BiometricType:      derefString(user.BiometricType),
			LastSecurityUpdate: formatTimePtr(user.LastSecurityUpdate),
		},
		WalletAddresses: walletAddrs,
		ActivitySummary: &pb.Web3UserActivitySummary{
			TotalTransactions:  txSummary.TotalTransactions,
			TotalDepositUsd:    txSummary.TotalDepositUSD,
			TotalWithdrawalUsd: txSummary.TotalWithdrawalUSD,
			TotalSwapUsd:       txSummary.TotalSwapUSD,
			FirstTransactionAt: formatTimePtr(txSummary.FirstTransactionAt),
			LastTransactionAt:  formatTimePtr(txSummary.LastTransactionAt),
		},
		CreatedAt:    formatTimePtr(user.CreatedAt),
		LastActiveAt: formatTimePtr(user.LastActiveAt),
	}

	return &pb.GetWeb3UserResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetWeb3UserData{
			User: d,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func toPBWeb3WalletAddress(m *model.Web3UserAddressModel) *pb.Web3WalletAddress {
	if m == nil {
		return nil
	}
	// Balance fields have been moved to web3_user_address_balances table
	// Using empty defaults until balance lookup is implemented
	return &pb.Web3WalletAddress{
		Network:           m.Network,
		ChainId:           m.ChainID,
		Address:           m.Address,
		IsPrimary:         m.IsPrimary,
		Enabled:           m.Enabled,
		IsBlacklisted:     m.IsBlacklisted,
		Balance:           "",
		BalanceUsd:        "",
		TokenBalances:     nil,
		CreatedAt:         formatTimePtr(m.CreatedAt),
		LastTransactionAt: formatTimePtr(m.LastTransactionAt),
		BlacklistReason:   derefString(m.BlacklistReason),
		RiskLevel:         derefString(m.RiskLevel),
		BlacklistedAt:     formatTimePtr(m.BlacklistedAt),
		BlacklistedBy:     derefString(m.BlacklistedBy),
	}
}
