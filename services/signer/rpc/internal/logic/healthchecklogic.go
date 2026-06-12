package logic

import (
	"context"
	"time"

	"internalwallet/common/constants"
	"internalwallet/common/errcode"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/models"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type HealthCheckLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHealthCheckLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HealthCheckLogic {
	return &HealthCheckLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *HealthCheckLogic) HealthCheck(in *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	// 1. 检查数据库连接
	dbConnected := true
	pingStart := time.Now()
	sqlDB, err := l.svcCtx.DB.DB()
	if err != nil {
		dbConnected = false
	} else {
		err = sqlDB.Ping()
		if err != nil {
			dbConnected = false
		}
	}
	pingMs := int32(time.Since(pingStart).Milliseconds())

	// 2. 查询Seed统计
	var totalSeeds int64
	var activeSeeds int64
	l.svcCtx.DB.Model(&models.MasterSeed{}).Count(&totalSeeds)
	l.svcCtx.DB.Model(&models.MasterSeed{}).Where("status = ?", constants.SeedStatusActive).Count(&activeSeeds)

	// 3. 确定整体状态
	status := "healthy"
	if !dbConnected {
		status = "unhealthy"
	}

	// 3.1 signer 解锁门禁状态（默认热钱包）
	walletState := walletStateUninitialized
	lockedReason := ""
	unlockedAt := ""
	if dbConnected {
		if snap, err := getHotWalletState(l.ctx, l.svcCtx, defaultHotWalletSeedID); err == nil && snap != nil {
			walletState = snap.State
			unlockedAt = snap.UnlockedAtRFC3339
			if walletState != walletStateUnlocked {
				lockedReason = errcode.GetSignerErrMsg(errcode.SignerWalletLocked)
			}
		} else if err != nil {
			l.Logger.Errorf("HealthCheck: getHotWalletState failed: %v", err)
		}
	}

	if status == "healthy" {
		if walletState != walletStateUnlocked {
			status = "degraded"
		} else if activeSeeds == 0 {
			status = "degraded"
		}
	}

	return &pb.HealthCheckResponse{
		Code:      0,
		Message:   "success",
		Status:    status,
		Version:   "v1.0.0",
		Timestamp: time.Now().Format(time.RFC3339),
		Seeds: &pb.SeedsHealth{
			Total:  int32(totalSeeds),
			Active: int32(activeSeeds),
		},
		Database: &pb.DatabaseHealth{
			Connected: dbConnected,
			PingMs:    pingMs,
		},
		WalletState:  walletState,
		LockedReason: lockedReason,
		UnlockedAt:   unlockedAt,
	}, nil
}
