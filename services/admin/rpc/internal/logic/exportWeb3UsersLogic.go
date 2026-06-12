package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
)

type ExportWeb3UsersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewExportWeb3UsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportWeb3UsersLogic {
	return &ExportWeb3UsersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ExportWeb3UsersLogic) ExportWeb3Users(in *pb.ExportWeb3UsersRequest) (*pb.ExportWeb3UsersResponse, error) {
	if in == nil {
		in = &pb.ExportWeb3UsersRequest{}
	}
	if l.svcCtx.DB == nil || l.svcCtx.Web3UserRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	format := strings.TrimSpace(in.Format)
	if format == "" {
		format = "xlsx"
	}
	if format != "xlsx" && format != "csv" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_FORMAT", "invalid format", map[string]string{"format": "invalid"})
	}

	createdFrom, createdTo, err := parseDateFromTo(in.CreatedFrom, in.CreatedTo)
	if err != nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_DATE", "invalid date", map[string]string{"created_from": "invalid"})
	}

	var twoFactorEnabled *bool
	if in.TwoFactorEnabled != nil {
		v := in.TwoFactorEnabled.Value
		twoFactorEnabled = &v
	}
	var biometricEnabled *bool
	if in.BiometricEnabled != nil {
		v := in.BiometricEnabled.Value
		biometricEnabled = &v
	}

	f := repository.Web3UserListFilter{
		AddressStatus:    strings.TrimSpace(in.AddressStatus),
		TwoFactorEnabled: twoFactorEnabled,
		BiometricEnabled: biometricEnabled,
		Network:          strings.TrimSpace(in.Network),
		Keyword:          strings.TrimSpace(in.Keyword),
		CreatedFrom:      createdFrom,
		CreatedTo:        createdTo,
	}
	summary, err := l.svcCtx.Web3UserRepo.Summary(l.ctx, f)
	if err != nil {
		l.Logger.Errorf("web3 users export summary failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	exportID := fmt.Sprintf("exp-web3-%s-%d", time.Now().UTC().Format("20060102-150405"), utils.GenerateID())
	estimatedTime := int32(60)
	if summary.TotalDevices > 5000 {
		estimatedTime = 180
	} else if summary.TotalDevices > 1000 {
		estimatedTime = 120
	}

	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"export_id":         exportID,
		"format":            format,
		"include_addresses": in.IncludeAddresses,
		"filters": map[string]interface{}{
			"address_status":     f.AddressStatus,
			"two_factor_enabled": twoFactorEnabled,
			"biometric_enabled":  biometricEnabled,
			"network":            f.Network,
			"keyword":            f.Keyword,
			"created_from":       in.CreatedFrom,
			"created_to":         in.CreatedTo,
		},
	})

	_ = l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		auditRepo := repository.NewAdminAuditLogRepository(tx)
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "web3users.export",
			TargetType:  "web3_user",
			TargetID:    exportID,
			Description: "导出 Web3 用户列表",
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	})

	return &pb.ExportWeb3UsersResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "EXPORT_TASK_CREATED"),
		Data: &pb.ExportWeb3UsersData{
			ExportId:         exportID,
			Status:           "processing",
			EstimatedRecords: summary.TotalDevices,
			EstimatedTime:    estimatedTime,
			DownloadUrl:      "",
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
