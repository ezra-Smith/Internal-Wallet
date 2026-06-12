package logic

import (
	"context"
	"errors"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/model"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type GetSwapDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSwapDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSwapDetailLogic {
	return &GetSwapDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetSwapDetail returns a swap transaction record stored in swap_svc_transactions.
func (l *GetSwapDetailLogic) GetSwapDetail(in *pb.GetSwapDetailRequest) (*pb.GetSwapDetailResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if l.svcCtx.TxRepo == nil {
		return nil, status.Error(codes.Internal, "db not ready")
	}

	txHash := strings.TrimSpace(in.TxHash)
	if txHash == "" {
		return nil, status.Error(codes.InvalidArgument, "tx_hash is required")
	}

	var rec *model.SwapSvcTransactionModel
	switch {
	case in.ChainId > 0:
		m, err := l.svcCtx.TxRepo.FindByTxHash(l.ctx, in.ChainId, txHash)
		if err != nil {
			// repo uses plain "not found" error strings for record miss
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				return nil, status.Error(codes.NotFound, "not found")
			}
			l.Errorf("find swap tx by hash failed: %v", err)
			return nil, status.Error(codes.Internal, "db error")
		}
		rec = m
	default:
		// Best-effort fallback: tx_hash may not be unique across chains; return latest record.
		var m model.SwapSvcTransactionModel
		err := l.svcCtx.TxRepo.GetDB().WithContext(l.ctx).
			Where("tx_hash = ?", txHash).
			Order("created_at DESC").
			First(&m).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, status.Error(codes.NotFound, "not found")
			}
			l.Errorf("find swap tx by hash (no chain_id) failed: %v", err)
			return nil, status.Error(codes.Internal, "db error")
		}
		rec = &m
	}

	// 查询 swap_configs 获取代币名称
	// 匹配条件：provider + chain_id + contract_address
	var fromTokenConfig, toTokenConfig model.SwapConfigModel

	// 查询 from_token 名称
	if rec.FromToken != "" {
		err := l.svcCtx.TxRepo.GetDB().WithContext(l.ctx).
			Where("provider = ? AND chain_id = ? AND contract_address = ?", rec.Provider, rec.ChainID, rec.FromToken).
			First(&fromTokenConfig).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("query from_token config failed: %v", err)
		}
	}

	// 查询 to_token 名称
	if rec.ToToken != "" {
		err := l.svcCtx.TxRepo.GetDB().WithContext(l.ctx).
			Where("provider = ? AND chain_id = ? AND contract_address = ?", rec.Provider, rec.ChainID, rec.ToToken).
			First(&toTokenConfig).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("query to_token config failed: %v", err)
		}
	}

	formatTime := func(t time.Time) string {
		return t.Local().Format(time.RFC3339)
	}
	formatTimePtr := func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return t.Local().Format(time.RFC3339)
	}
	errMsg := ""
	if rec.ErrorMessage != nil {
		errMsg = strings.TrimSpace(*rec.ErrorMessage)
	}

	return &pb.GetSwapDetailResponse{
		Success: true,
		Message: "ok",
		Data: &pb.SwapDetailData{
			Id:            rec.ID,
			ApiKeyId:      rec.ApiKeyID,
			ProjectName:   rec.ProjectName,
			WalletAddress: rec.WalletAddress,
			ChainId:       rec.ChainID,
			Provider:      rec.Provider,

			FromToken:  rec.FromToken,
			ToToken:    rec.ToToken,
			FromAmount: rec.FromAmount,
			ToAmount:   rec.ToAmount,

			TxFrom:          rec.TxFrom,
			TxTo:            rec.TxTo,
			TxData:          rec.TxData,
			TxValue:         rec.TxValue,
			TxGas:           rec.TxGas,
			TxGasPrice:      rec.TxGasPrice,
			TxNonce:         rec.TxNonce,
			TxSignatureData: rec.TxSignatureData,

			TxHash:        rec.TxHash,
			Status:        rec.Status,
			ErrorCode:     rec.ErrorCode,
			ErrorMessage:  errMsg,
			BroadcastedAt: formatTimePtr(rec.BroadcastedAt),

			CreatedAt:     formatTime(rec.CreatedAt),
			UpdatedAt:     formatTime(rec.UpdatedAt),
			FromTokenName: fromTokenConfig.TokenName,
			ToTokenName:   toTokenConfig.TokenName,
		},
	}, nil
}
