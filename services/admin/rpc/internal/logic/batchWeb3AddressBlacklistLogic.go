package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type BatchWeb3AddressBlacklistLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBatchWeb3AddressBlacklistLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchWeb3AddressBlacklistLogic {
	return &BatchWeb3AddressBlacklistLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BatchWeb3AddressBlacklistLogic) BatchWeb3AddressBlacklist(in *pb.BatchWeb3AddressBlacklistRequest) (*pb.BatchWeb3AddressBlacklistResponse, error) {
	if in == nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid request", nil)
	}
	action := strings.TrimSpace(in.Action)
	if action != "add" && action != "remove" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ACTION", "invalid action", map[string]string{"action": "invalid"})
	}
	if len(in.Addresses) == 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "addresses required", map[string]string{"addresses": "required"})
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "reason required", map[string]string{"reason": "required"})
	}
	riskLevel := strings.TrimSpace(in.RiskLevel)
	if action == "add" && (riskLevel == "" || !validateRiskLevel(riskLevel)) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_RISK_LEVEL", "invalid risk_level", map[string]string{"risk_level": "invalid"})
	}

	results := make([]*pb.BatchWeb3AddressResult, 0, len(in.Addresses))
	var succeeded int32
	for _, a := range in.Addresses {
		if a == nil {
			continue
		}
		deviceID := strings.TrimSpace(a.DeviceId)
		addr := strings.TrimSpace(a.Address)
		if deviceID == "" || addr == "" {
			results = append(results, &pb.BatchWeb3AddressResult{
				DeviceId: deviceID,
				Address:  maskAddress(addr),
				Status:   "failed",
				Error:    "device_id/address required",
			})
			continue
		}

		if action == "add" {
			_, err := NewBlacklistWeb3AddressLogic(l.ctx, l.svcCtx).BlacklistWeb3Address(&pb.BlacklistWeb3AddressRequest{
				DeviceId:       deviceID,
				Address:        addr,
				Reason:         reason,
				RiskLevel:      riskLevel,
				DisableAddress: false,
				NotifyUser:     in.NotifyUsers,
			})
			if err != nil {
				results = append(results, &pb.BatchWeb3AddressResult{
					DeviceId: deviceID,
					Address:  maskAddress(addr),
					Status:   "failed",
					Error:    err.Error(),
				})
				continue
			}
		} else {
			_, err := NewUnblacklistWeb3AddressLogic(l.ctx, l.svcCtx).UnblacklistWeb3Address(&pb.UnblacklistWeb3AddressRequest{
				DeviceId:      deviceID,
				Address:       addr,
				Reason:        reason,
				EnableAddress: false,
				NotifyUser:    in.NotifyUsers,
			})
			if err != nil {
				results = append(results, &pb.BatchWeb3AddressResult{
					DeviceId: deviceID,
					Address:  maskAddress(addr),
					Status:   "failed",
					Error:    err.Error(),
				})
				continue
			}
		}

		succeeded++
		results = append(results, &pb.BatchWeb3AddressResult{
			DeviceId: deviceID,
			Address:  maskAddress(addr),
			Status:   "success",
			Error:    "",
		})
	}

	total := int32(len(in.Addresses))
	failed := total - succeeded
	return &pb.BatchWeb3AddressBlacklistResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "BATCH_BULK_DONE"),
		Data: &pb.BatchWeb3AddressBlacklistData{
			Total:     total,
			Succeeded: succeeded,
			Failed:    failed,
			Results:   results,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
