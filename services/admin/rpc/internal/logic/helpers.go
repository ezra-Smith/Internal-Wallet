package logic

import (
	"encoding/json"
	"math"
	"strings"
	"time"

	"internalwallet/proto/pb"
)

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func formatTimePtr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// formatTimeFriendly 返回友好的时间格式 yyyy-MM-dd HH:mm:ss
func formatTimeFriendly(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// formatTimePtrFriendly 返回友好的时间格式 yyyy-MM-dd HH:mm:ss
func formatTimePtrFriendly(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func calcPagination(page, pageSize int32, total int64) *pb.Pagination {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if total < 0 {
		total = 0
	}

	totalPages := int32(0)
	if total > 0 {
		totalPages = int32(math.Ceil(float64(total) / float64(pageSize)))
	}

	hasPrev := page > 1 && totalPages > 0
	hasNext := totalPages > 0 && page < totalPages

	return &pb.Pagination{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    hasNext,
		HasPrev:    hasPrev,
	}
}

func toJSONString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

func maskPhone(phone string) string {
	s := strings.TrimSpace(phone)
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		if len(s) <= 4 {
			return "****"
		}
		return "****" + s[len(s)-4:]
	}
	return s[:len(s)-8] + "****" + s[len(s)-4:]
}

func maskIDNumber(idNumber string) string {
	s := strings.TrimSpace(idNumber)
	if s == "" {
		return ""
	}
	if len(s) <= 7 {
		return "****"
	}
	return s[:3] + strings.Repeat("*", len(s)-7) + s[len(s)-4:]
}

func maskEmail(email string) string {
	s := strings.TrimSpace(email)
	if s == "" {
		return ""
	}
	at := strings.Index(s, "@")
	if at <= 0 || at >= len(s)-1 {
		return s
	}
	local := s[:at]
	domain := s[at+1:]
	if domain == "" {
		return s
	}

	// local part masking:
	// 1 -> "*"
	// 2 -> "a*"
	// 3-4 -> "a**b"
	// >=5 -> "ab***c"
	var maskedLocal string
	switch {
	case len(local) <= 1:
		maskedLocal = "*"
	case len(local) == 2:
		maskedLocal = local[:1] + "*"
	case len(local) <= 4:
		maskedLocal = local[:1] + strings.Repeat("*", len(local)-2) + local[len(local)-1:]
	default:
		maskedLocal = local[:2] + "***" + local[len(local)-1:]
	}
	return maskedLocal + "@" + domain
}

// 智能脱敏昵称：
// - 若 nickname 等于 phone（含/不含国家码）则返回手机号掩码
// - 若 nickname 等于 email（忽略大小写）则返回邮箱掩码
// - 否则返回原昵称（trim 后）
func maskNicknameIfDefault(nickname, phone, countryCode, email string) string {
	n := strings.TrimSpace(nickname)
	if n == "" {
		return ""
	}

	p := strings.TrimSpace(phone)
	if p != "" {
		full := fullPhone(countryCode, p)
		fullNoPlus := strings.TrimPrefix(full, "+")
		if n == p || n == full || n == fullNoPlus {
			// Use the local phone (DB field) to keep output shape close to 131****8888.
			return maskPhone(p)
		}
	}

	e := strings.TrimSpace(email)
	if e != "" && strings.EqualFold(n, e) {
		return maskEmail(e)
	}

	return n
}

func fullPhone(countryCode, phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ""
	}
	if strings.HasPrefix(phone, "+") {
		return phone
	}
	cc := strings.TrimSpace(countryCode)
	cc = strings.TrimPrefix(cc, "+")
	if cc == "" {
		return phone
	}
	return "+" + cc + phone
}

func userRoleFromMemberLevel(level int32) string {
	if level >= 2 {
		return "vip"
	}
	return "user"
}

func userKycStatusFromLevel(level int32) string {
	if level > 0 {
		return "verified"
	}
	return "pending"
}

func userStatusFrom(status int32, kycLevel int32) string {
	switch status {
	case 2:
		return "frozen"
	case 3:
		return "terminated"
	default:
		if kycLevel <= 0 {
			return "pending_kyc"
		}
		return "active"
	}
}
