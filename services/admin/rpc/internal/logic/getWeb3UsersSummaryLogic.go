package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetWeb3UsersSummaryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWeb3UsersSummaryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWeb3UsersSummaryLogic {
	return &GetWeb3UsersSummaryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetWeb3UsersSummaryLogic) GetWeb3UsersSummary(in *pb.GetWeb3UsersSummaryRequest) (*pb.GetWeb3UsersSummaryResponse, error) {
	if l.svcCtx.DB == nil || l.svcCtx.Web3UserRepo == nil || l.svcCtx.Web3UserAddressRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	// Get total device count (Web3 users)
	totalDevices, err := l.svcCtx.Web3UserRepo.CountAll(l.ctx)
	if err != nil {
		l.Logger.Errorf("count all web3 users failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query failed", nil)
	}

	// Get total address count
	totalAddresses, err := l.svcCtx.Web3UserAddressRepo.CountAll(l.ctx)
	if err != nil {
		l.Logger.Errorf("count all addresses failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query failed", nil)
	}

	// Get blacklisted address count
	blacklistedAddresses, err := l.svcCtx.Web3UserAddressRepo.CountBlacklisted(l.ctx)
	if err != nil {
		l.Logger.Errorf("count blacklisted addresses failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query failed", nil)
	}

	// Get 2FA enabled user count
	twoFactorEnabled, err := l.svcCtx.Web3UserRepo.Count2FAEnabled(l.ctx)
	if err != nil {
		l.Logger.Errorf("count 2FA enabled users failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query failed", nil)
	}

	// Calculate 2FA enabled rate
	var twoFactorRate float64
	if totalDevices > 0 {
		twoFactorRate = float64(twoFactorEnabled) / float64(totalDevices) * 100
	}

	return &pb.GetWeb3UsersSummaryResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetWeb3UsersSummaryData{
			TotalDevices:         totalDevices,
			TotalAddresses:       totalAddresses,
			BlacklistedAddresses: blacklistedAddresses,
			TwoFactorEnabledRate: twoFactorRate,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
