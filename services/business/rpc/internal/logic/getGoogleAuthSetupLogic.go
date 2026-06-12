package logic

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/skip2/go-qrcode"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetGoogleAuthSetupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGoogleAuthSetupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGoogleAuthSetupLogic {
	return &GetGoogleAuthSetupLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetGoogleAuthSetupLogic) GetGoogleAuthSetup(in *pb.GetGoogleAuthSetupReq) (*pb.GetGoogleAuthSetupResp, error) {
	_ = in

	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil {
		return nil, errx.InvalidUser()
	}

	// 检查用户是否已绑定 Google 验证器
	user, err := l.svcCtx.UserAccountRepository.GetAuthByID(l.ctx, uid)
	if err != nil || user == nil {
		return nil, errx.UserNotFound()
	}
	if user.Is2faEnabled && strings.TrimSpace(user.GoogleAuthSecret) != "" {
		return nil, errx.Conflict("已绑定 Google 验证器，如需重新绑定请先解绑")
	}

	// 生成 TOTP secret
	secret, err := generateTOTPSecret(20)
	if err != nil {
		l.Logger.Errorf("generate totp secret failed: %v", err)
		return nil, errx.GenerateFailed()
	}

	// 构建账户名称（优先使用邮箱，其次手机号）
	accountName := ""
	if user.Email != "" {
		accountName = user.Email
	} else if user.Phone != "" {
		accountName = user.Phone
	} else {
		accountName = fmt.Sprintf("user_%d", uid)
	}

	// 构建 otpauth URL
	issuer := "Zink Wallet"
	otpauthURL := buildTOTPAuthURL(issuer, accountName, secret)
	if otpauthURL == "" {
		return nil, errx.GenerateFailed()
	}

	// 生成 QR 码
	png, err := qrcode.Encode(otpauthURL, qrcode.Medium, 256)
	if err != nil {
		l.Logger.Errorf("qrcode encode failed: %v", err)
		return nil, errx.GenerateFailed()
	}
	qrDataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)

	return &pb.GetGoogleAuthSetupResp{
		Success:     true,
		Message:     "ok",
		Secret:      secret,
		QrCode:      qrDataURI,
		OtpauthUrl:  otpauthURL,
		AccountName: accountName,
	}, nil
}

// generateTOTPSecret 生成 Base32 编码的 TOTP secret
func generateTOTPSecret(numBytes int) (string, error) {
	if numBytes <= 0 {
		numBytes = 20
	}
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	return enc.EncodeToString(b), nil
}

// buildTOTPAuthURL 构建 otpauth URL
func buildTOTPAuthURL(issuer, account, secret string) string {
	issuer = strings.TrimSpace(issuer)
	account = strings.TrimSpace(account)
	secret = strings.TrimSpace(secret)
	if issuer == "" || account == "" || secret == "" {
		return ""
	}

	label := url.PathEscape(fmt.Sprintf("%s:%s", issuer, account))
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", "6")
	q.Set("period", "30")
	return fmt.Sprintf("otpauth://totp/%s?%s", label, q.Encode())
}
