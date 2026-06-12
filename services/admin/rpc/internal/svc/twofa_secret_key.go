package svc

import (
	"os"
	"strings"
)

// TwoFASecretKeyMaterial returns key material for encrypting/decrypting TOTP secrets in DB.
// Priority:
// 1) ENV ADMIN_2FA_SECRET_ENC_KEY
// 2) Config.Jwt.RefreshSecret (available in all deployments)
func (s *ServiceContext) TwoFASecretKeyMaterial() string {
	if v := strings.TrimSpace(os.Getenv("ADMIN_2FA_SECRET_ENC_KEY")); v != "" {
		return v
	}
	return strings.TrimSpace(s.Config.JWT.RefreshSecret)
}
