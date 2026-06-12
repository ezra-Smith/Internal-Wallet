package model

import (
	"internalwallet/common/model"
)

// NotificationTemplate 推送模板模型（对应 notification_templates 表）
type NotificationTemplate struct {
	model.BaseModel

	TemplateCode     string `gorm:"column:template_code;size:50;not null;uniqueIndex:uk_notification_templates_code_lang" json:"template_code"`
	TemplateName     string `gorm:"column:template_name;size:100;not null" json:"template_name"`
	NotificationType string `gorm:"column:notification_type;size:20;not null;index:idx_notification_templates_type" json:"notification_type"`
	Language         string `gorm:"column:language;size:10;not null;default:'zh-CN';uniqueIndex:uk_notification_templates_code_lang" json:"language"`
	TitleTemplate    string `gorm:"column:title_template;size:200;not null" json:"title_template"`
	ContentTemplate  string `gorm:"column:content_template;type:text;not null" json:"content_template"`
	Priority         int    `gorm:"column:priority;not null;default:1" json:"priority"` // 1=低 2=中 3=高
	IsEnabled        bool   `gorm:"column:is_enabled;not null;default:true;index:idx_notification_templates_is_enabled" json:"is_enabled"`
	Description      string `gorm:"column:description;size:500;not null;default:''" json:"description"`
}

// TableName 指定表名
func (NotificationTemplate) TableName() string {
	return "notification_templates"
}

// IsHighPriority 判断是否为高优先级
func (t *NotificationTemplate) IsHighPriority() bool {
	return t.Priority >= 3
}
