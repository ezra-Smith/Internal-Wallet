package models

import (
	"internalwallet/common/model"
)

type ConsolidationLog struct {
	model.BaseModel

	TaskID   string `gorm:"column:task_id;size:64;index:idx_consolidation_logs_task_id;not null" json:"task_id"`
	LogLevel string `gorm:"column:log_level;size:10;not null" json:"log_level"`
	Message  string `gorm:"column:message;type:text;not null" json:"message"`
	Details  []byte `gorm:"column:details;type:json" json:"details,omitempty"`
}

func (ConsolidationLog) TableName() string {
	return "consolidation_logs"
}
