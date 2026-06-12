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

type GetDashboardActivitiesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDashboardActivitiesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDashboardActivitiesLogic {
	return &GetDashboardActivitiesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetDashboardActivitiesLogic) GetDashboardActivities(in *pb.GetDashboardActivitiesRequest) (*pb.GetDashboardActivitiesResponse, error) {
	if in == nil {
		in = &pb.GetDashboardActivitiesRequest{}
	}
	if l.svcCtx.DB == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	// 分页参数
	page := in.Page
	if page <= 0 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize

	// 查询操作动态
	activities, err := l.fetchRecentActivities(pageSize, offset)
	if err != nil {
		l.Logger.Errorf("fetch recent activities failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "failed to fetch activities", nil)
	}

	// 查询总数
	total, err := l.countTotalActivities()
	if err != nil {
		l.Logger.Errorf("count total activities failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "failed to count activities", nil)
	}

	return &pb.GetDashboardActivitiesResponse{
		Success: true,
		Message: "ok",
		Data: &pb.DashboardActivitiesData{
			Activities: activities,
			Total:      total,
			Page:       page,
			PageSize:   pageSize,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

// fetchRecentActivities 查询最近操作动态
// 修复：
// 1. currency_withdraw_orders 使用 id 而不是 order_id
// 2. currency_withdraw_orders 使用 asset_code 而不是 currency
// 3. 所有字符串字段添加 COLLATE utf8mb4_unicode_ci 以统一字符集
// 4. UNIX_TIMESTAMP 结果转换为 BIGINT
func (l *GetDashboardActivitiesLogic) fetchRecentActivities(pageSize, offset int32) ([]*pb.RecentActivity, error) {
	type activityRow struct {
		ActivityType string `gorm:"column:activity_type"`
		Title        string `gorm:"column:title"`
		Description  string `gorm:"column:description"`
		RelatedID    string `gorm:"column:related_id"`
		CreatedAt    int64  `gorm:"column:created_at"`
		Operator     string `gorm:"column:operator"`
		StatusIcon   string `gorm:"column:status_icon"`
	}

	var rows []activityRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw(`
			(
				SELECT 
					'transfer_batch' COLLATE utf8mb4_unicode_ci AS activity_type,
					CONCAT('转账批次: ', batch_id) COLLATE utf8mb4_unicode_ci AS title,
					CONCAT('总计: ', total_recipients, ' 笔, 金额: ', total_amount_usd, ' USD') COLLATE utf8mb4_unicode_ci AS description,
					CAST(batch_id AS CHAR) COLLATE utf8mb4_unicode_ci AS related_id,
					CAST(UNIX_TIMESTAMP(created_at) AS SIGNED) AS created_at,
					COALESCE(created_by, '') COLLATE utf8mb4_unicode_ci AS operator,
					CASE 
						WHEN status='completed' THEN '✓' COLLATE utf8mb4_unicode_ci
						WHEN status='failed' OR status='partial_failed' THEN '✗' COLLATE utf8mb4_unicode_ci
						ELSE '⧗' COLLATE utf8mb4_unicode_ci
					END AS status_icon
				FROM transfer_batches
				ORDER BY created_at DESC
				LIMIT 50
			)
			UNION ALL
			(
				SELECT 
					'withdraw_approval' COLLATE utf8mb4_unicode_ci AS activity_type,
					CONCAT('提现审批: ', id) COLLATE utf8mb4_unicode_ci AS title,
					CONCAT('金额: ', amount, ' ', asset_code) COLLATE utf8mb4_unicode_ci AS description,
					CAST(id AS CHAR) COLLATE utf8mb4_unicode_ci AS related_id,
					CAST(UNIX_TIMESTAMP(created_at) AS SIGNED) AS created_at,
					CASE 
						WHEN updated_by > 0 THEN CAST(updated_by AS CHAR)
						ELSE '系统'
					END COLLATE utf8mb4_unicode_ci AS operator,
					CASE 
						WHEN status='completed' THEN '✓' COLLATE utf8mb4_unicode_ci
						WHEN status='failed' THEN '✗' COLLATE utf8mb4_unicode_ci
						WHEN status='pending' OR status='processing' THEN '⧗' COLLATE utf8mb4_unicode_ci
						ELSE '-' COLLATE utf8mb4_unicode_ci
					END AS status_icon
				FROM currency_withdraw_orders
				WHERE deleted_at IS NULL
				ORDER BY created_at DESC
				LIMIT 50
			)
			UNION ALL
			(
				SELECT 
					'user_registration' COLLATE utf8mb4_unicode_ci AS activity_type,
					CONCAT('新用户注册: ', COALESCE(email, phone, nickname, CAST(id AS CHAR))) COLLATE utf8mb4_unicode_ci AS title,
					CONCAT('用户ID: ', id) COLLATE utf8mb4_unicode_ci AS description,
					CAST(id AS CHAR) COLLATE utf8mb4_unicode_ci AS related_id,
					CAST(UNIX_TIMESTAMP(created_at) AS SIGNED) AS created_at,
					'系统' COLLATE utf8mb4_unicode_ci AS operator,
					'✓' COLLATE utf8mb4_unicode_ci AS status_icon
				FROM users
				WHERE deleted_at IS NULL
				ORDER BY created_at DESC
				LIMIT 50
			)
			UNION ALL
			(
				SELECT 
					'vault_transfer' COLLATE utf8mb4_unicode_ci AS activity_type,
					CONCAT('金库调整: ', COALESCE(currency, 'N/A')) COLLATE utf8mb4_unicode_ci AS title,
					CONCAT('金额: ', amount, ' ', COALESCE(currency, ''), ', 类型: ', adjustment_type) COLLATE utf8mb4_unicode_ci AS description,
					CAST(id AS CHAR) COLLATE utf8mb4_unicode_ci AS related_id,
					CAST(UNIX_TIMESTAMP(created_at) AS SIGNED) AS created_at,
					CASE 
						WHEN reviewed_by IS NOT NULL THEN CAST(reviewed_by AS CHAR)
						ELSE CAST(submitted_by AS CHAR)
					END COLLATE utf8mb4_unicode_ci AS operator,
					'✓' COLLATE utf8mb4_unicode_ci AS status_icon
				FROM vault_adjustments
				WHERE deleted_at IS NULL
				ORDER BY created_at DESC
				LIMIT 50
			)
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?
		`, pageSize, offset).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]*pb.RecentActivity, 0, len(rows))
	for _, row := range rows {
		result = append(result, &pb.RecentActivity{
			ActivityType: row.ActivityType,
			Title:        row.Title,
			Description:  row.Description,
			RelatedId:    row.RelatedID,
			CreatedAt:    row.CreatedAt,
			Operator:     row.Operator,
			StatusIcon:   row.StatusIcon,
		})
	}

	return result, nil
}

// countTotalActivities 统计操作动态总数
func (l *GetDashboardActivitiesLogic) countTotalActivities() (int32, error) {
	type countRow struct {
		Total int32 `gorm:"column:total"`
	}

	var result countRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw(`
			SELECT (
				(SELECT COUNT(*) FROM transfer_batches LIMIT 50) +
				(SELECT COUNT(*) FROM currency_withdraw_orders WHERE deleted_at IS NULL LIMIT 50) +
				(SELECT COUNT(*) FROM users WHERE deleted_at IS NULL LIMIT 50) +
				(SELECT COUNT(*) FROM vault_adjustments WHERE deleted_at IS NULL LIMIT 50)
			) AS total
		`).
		Scan(&result).Error; err != nil {
		return 0, err
	}

	// 最多返回200条
	if result.Total > 200 {
		result.Total = 200
	}

	return result.Total, nil
}
