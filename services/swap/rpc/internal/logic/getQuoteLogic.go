package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/provider"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetQuoteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetQuoteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetQuoteLogic {
	return &GetQuoteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== Swap APIs ====================
func (l *GetQuoteLogic) GetQuote(in *pb.GetQuoteRequest) (*pb.GetQuoteResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if in.ChainId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "chain_id is required")
	}
	if !isValidTokenAddress(in.FromTokenAddress) {
		return nil, status.Error(codes.InvalidArgument, "invalid from_token_address")
	}
	if !isValidTokenAddress(in.ToTokenAddress) {
		return nil, status.Error(codes.InvalidArgument, "invalid to_token_address")
	}
	amt, err := parseUintDecimal(in.Amount)
	if err != nil || amt.Sign() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid amount")
	}
	if l.svcCtx.Provider == nil {
		return nil, status.Error(codes.Internal, "swap provider not ready")
	}

	res, err := l.svcCtx.Provider.GetQuote(l.ctx, &provider.QuoteRequest{
		ChainID:          in.ChainId,
		FromTokenAddress: strings.TrimSpace(in.FromTokenAddress),
		ToTokenAddress:   strings.TrimSpace(in.ToTokenAddress),
		Amount:           strings.TrimSpace(in.Amount),
		IncludeProtocols: in.IncludeProtocols,
	})
	if err != nil {
		return &pb.GetQuoteResponse{
			Success: false,
			Message: err.Error(),
			Data:    nil,
		}, nil
	}

	return &pb.GetQuoteResponse{
		Success: true,
		Message: "ok",
		Data:    providerQuoteToPB(res),
	}, nil
}
