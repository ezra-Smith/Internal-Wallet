package logic

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	mysqlDriver "github.com/go-sql-driver/mysql"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/model"
)

func toAcctAssetPB(m *model.AssetModel) *pb.AcctAsset {
	if m == nil {
		return nil
	}
	code := strings.ToUpper(strings.TrimSpace(m.Code))
	name := strings.TrimSpace(m.Name)
	if name == "" {
		name = code
	}
	return &pb.AcctAsset{
		Code:      code,
		Name:      name,
		IconUrl:   strings.TrimSpace(m.IconUrl),
		Precision: m.Precision,
		Status:    int32(m.Status),
		IsHot:     m.IsHot,
	}
}

func normalizeAssetCode(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func isValidAssetCode(code string) bool {
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 32 {
		return false
	}
	for _, r := range code {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func validateHTTPIconURL(iconURL string) error {
	iconURL = strings.TrimSpace(iconURL)
	if iconURL == "" {
		return nil
	}
	if len(iconURL) > 2048 {
		return fmt.Errorf("icon_url too long")
	}
	u, err := url.ParseRequestURI(iconURL)
	if err != nil || u == nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("invalid icon_url")
	}
	return nil
}

func isMySQLDuplicate(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
