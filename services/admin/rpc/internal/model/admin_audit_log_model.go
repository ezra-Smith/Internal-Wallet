package model

import (
	commonModel "internalwallet/common/model"
)

// AdminAuditLogModel 管理员审计日志
type AdminAuditLogModel struct {
	commonModel.BaseModel

	AdminID int64 `gorm:"column:admin_id;type:bigint;not null;index"`

	Action     string `gorm:"column:action;type:varchar(64);not null;index"`
	TargetType string `gorm:"column:target_type;type:varchar(32);not null;index"`
	TargetID   string `gorm:"column:target_id;type:varchar(64);default:'';index"`

	Description string `gorm:"column:description;type:varchar(512);not null"`
	// details 使用 JSON 字段存储（序列化为 []byte）
	Details []byte `gorm:"column:details;type:json"`

	IP        string `gorm:"column:ip;type:varchar(45);not null"`
	UserAgent string `gorm:"column:user_agent;type:varchar(512);default:''"`

	// ============
	// v2 fields (request-level audit)
	// ============
	Success        bool   `gorm:"column:success;type:tinyint(1);not null;default:1;index"`
	GrpcCode       int32  `gorm:"column:grpc_code;type:int;not null;default:0"`
	ErrorMessage   string `gorm:"column:error_message;type:varchar(1024);not null;default:''"`
	RequestID      string `gorm:"column:request_id;type:varchar(64);not null;default:'';index"`
	RpcMethod      string `gorm:"column:rpc_method;type:varchar(128);not null;default:'';index"`
	HttpMethod     string `gorm:"column:http_method;type:varchar(16);not null;default:''"`
	HttpPath       string `gorm:"column:http_path;type:varchar(256);not null;default:''"`
	DurationMs     int32  `gorm:"column:duration_ms;type:int;not null;default:0"`
	OperatorEmail  string `gorm:"column:operator_email;type:varchar(128);not null;default:'';index"`
	IdempotencyKey string `gorm:"column:idempotency_key;type:varchar(128);not null;default:''"`
	Module         string `gorm:"column:module;type:varchar(32);not null;default:'';index"`
}

func (AdminAuditLogModel) TableName() string { return "admin_audit_logs" }
