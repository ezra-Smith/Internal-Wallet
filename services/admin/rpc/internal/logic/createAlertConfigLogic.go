package logic

import (
	"context"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"

	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/svc"
)

type CreateAlertConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateAlertConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateAlertConfigLogic {
	return &CreateAlertConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateAlertConfigLogic) CreateAlertConfig(in *pb.AdminCreateAlertConfigRequest) (*pb.AdminCreateAlertConfigResponse, error) {
	// 获取当前管理员ID
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		l.Logger.Errorf("Failed to get current admin from context")
		return &pb.AdminCreateAlertConfigResponse{
			Success: false,
			Message: "获取管理员信息失败",
		}, nil
	}

	// 1. 参数验证
	if err := l.validateRequest(in); err != nil {
		l.Logger.Errorf("Invalid request: %v", err)
		return &pb.AdminCreateAlertConfigResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// 2. 检查名称是否重复
	existingConfig, err := l.svcCtx.AlertConfigRepo.FindByName(l.ctx, in.Name)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Logger.Errorf("Failed to check existing config: %v", err)
		return &pb.AdminCreateAlertConfigResponse{
			Success: false,
			Message: "Failed to check existing config",
		}, err
	}
	if existingConfig != nil {
		return &pb.AdminCreateAlertConfigResponse{
			Success: false,
			Message: fmt.Sprintf("Alert config with name '%s' already exists", in.Name),
		}, nil
	}

	// 3. 转换阈值
	thresholdUSD, err := decimal.NewFromString(in.ThresholdUsd)
	if err != nil {
		return &pb.AdminCreateAlertConfigResponse{
			Success: false,
			Message: "Invalid threshold_usd format",
		}, nil
	}

	// 4. 生成雪花ID
	configID := utils.GenerateID()

	// 5. 创建配置模型
	config := &model.AlertConfigModel{
		Name:                    in.Name,
		Description:             in.Description,
		AlertType:               "platform_transaction",
		ThresholdUSD:            thresholdUSD,
		TimeWindowSeconds:       int(in.TimeWindowSeconds),
		MonitorWeb3Withdraw:     in.MonitorWeb3Withdraw,
		MonitorWeb2Withdraw:     in.MonitorWeb2Withdraw,
		MonitorInternalTransfer: in.MonitorInternalTransfer,
		Enabled:                 in.Enabled,
		TestMode:                in.TestMode,
		CooldownSeconds:         int(in.CooldownSeconds),
		CreatedBy:               current.ID,
		UpdatedBy:               current.ID,
	}
	config.ID = configID

	// 6. 保存到数据库
	if err := l.svcCtx.AlertConfigRepo.Create(l.ctx, config); err != nil {
		l.Logger.Errorf("Failed to create alert config: %v", err)
		return &pb.AdminCreateAlertConfigResponse{
			Success: false,
			Message: "Failed to create alert config",
		}, err
	}

	l.Logger.Infof("Alert config created successfully: id=%d, name=%s", configID, in.Name)

	// 7. 返回结果
	return &pb.AdminCreateAlertConfigResponse{
		Success:  true,
		Message:  "Alert config created successfully",
		ConfigId: configID,
		Config:   l.modelToProto(config),
	}, nil
}

// validateRequest 验证请求参数
func (l *CreateAlertConfigLogic) validateRequest(in *pb.AdminCreateAlertConfigRequest) error {
	// 名称不能为空
	if in.Name == "" {
		return errors.New("name is required")
	}

	// 阈值必须大于0
	threshold, err := decimal.NewFromString(in.ThresholdUsd)
	if err != nil || threshold.LessThanOrEqual(decimal.Zero) {
		return errors.New("threshold_usd must be greater than 0")
	}

	// 时间窗口至少60秒
	if in.TimeWindowSeconds < 60 {
		return errors.New("time_window_seconds must be at least 60 seconds")
	}

	// 冷却时间不能为负
	if in.CooldownSeconds < 0 {
		return errors.New("cooldown_seconds cannot be negative")
	}

	// 至少启用一种监控类型
	if !in.MonitorWeb3Withdraw && !in.MonitorWeb2Withdraw && !in.MonitorInternalTransfer {
		return errors.New("at least one monitor type must be enabled")
	}

	return nil
}

// modelToProto 将模型转换为Proto消息
func (l *CreateAlertConfigLogic) modelToProto(config *model.AlertConfigModel) *pb.AdminAlertConfigItem {
	return &pb.AdminAlertConfigItem{
		Id:                      config.ID,
		Name:                    config.Name,
		Description:             config.Description,
		AlertType:               config.AlertType,
		ThresholdUsd:            config.ThresholdUSD.String(),
		TimeWindowSeconds:       int32(config.TimeWindowSeconds),
		MonitorWeb3Withdraw:     config.MonitorWeb3Withdraw,
		MonitorWeb2Withdraw:     config.MonitorWeb2Withdraw,
		MonitorInternalTransfer: config.MonitorInternalTransfer,
		Enabled:                 config.Enabled,
		TestMode:                config.TestMode,
		CooldownSeconds:         int32(config.CooldownSeconds),
		CreatedAt:               config.CreatedAt.Unix(),
		UpdatedAt:               config.UpdatedAt.Unix(),
	}
}
