package model

import commonModel "internalwallet/common/model"

// MemberBiometricCredentialModel stores Web2 biometric public keys for signature-based authentication.
// Table: member_biometric_credential
type MemberBiometricCredentialModel struct {
	commonModel.BaseModel

	UserId int64  `gorm:"column:user_id;type:bigint;not null;index;comment:用户ID"`
	KeyId  string `gorm:"column:key_id;type:varchar(128);not null;comment:客户端 key_id"`

	Algorithm       string `gorm:"column:algorithm;type:varchar(32);not null;default:'ES256';comment:算法"`
	PublicKeyFormat string `gorm:"column:public_key_format;type:varchar(32);not null;default:'';comment:公钥格式"`
	PublicKey       []byte `gorm:"column:public_key;type:varbinary(512);not null;comment:公钥原始字节"`

	Status int32 `gorm:"column:status;type:tinyint;not null;default:1;comment:1=启用 0=禁用"`

	Platform   string `gorm:"column:platform;type:varchar(32);not null;default:'';comment:平台"`
	DeviceId   string `gorm:"column:device_id;type:varchar(128);not null;default:'';index;comment:设备ID"`
	AppVersion string `gorm:"column:app_version;type:varchar(32);not null;default:'';comment:APP版本"`
}

func (MemberBiometricCredentialModel) TableName() string { return "member_biometric_credential" }

