package captcha

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"

	"github.com/mojocn/base64Captcha"
)

// SecureDigitGenerator generates digit captchas with crypto-secure ids and answers.
// It intentionally does not use base64Captcha.RandomId / randomDigits (math/rand).
//
// Note: The image distortions inside base64Captcha's digit drawing still rely on math/rand,
// but the captcha key and answer generation here are cryptographically secure.
type SecureDigitGenerator struct {
	store  Store
	driver *base64Captcha.DriverDigit
	length int
}

func NewSecureDigitGenerator(store Store) *SecureDigitGenerator {
	driver := base64Captcha.NewDriverDigit(
		80,  // height
		240, // width
		6,   // length
		0.7, // maxSkew
		80,  // dotCount
	)
	return &SecureDigitGenerator{
		store:  store,
		driver: driver,
		length: 6,
	}
}

func NewCustomSecureDigitGenerator(store Store, height, width, length int, maxSkew float64, dotCount int) *SecureDigitGenerator {
	if length <= 0 {
		length = 6
	}
	driver := base64Captcha.NewDriverDigit(height, width, length, maxSkew, dotCount)
	return &SecureDigitGenerator{
		store:  store,
		driver: driver,
		length: length,
	}
}

// Generate returns captcha_id and base64 image, storing the answer server-side with TTL.
func (g *SecureDigitGenerator) Generate(ctx context.Context) (*CaptchaResponse, error) {
	if g == nil || g.store == nil || g.driver == nil {
		return nil, fmt.Errorf("captcha generator not initialized")
	}

	id, err := generateSecureKey(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate captcha id: %w", err)
	}
	answer, err := cryptoDigits(g.length)
	if err != nil {
		return nil, fmt.Errorf("failed to generate captcha answer: %w", err)
	}

	item, err := g.driver.DrawCaptcha(answer)
	if err != nil {
		return nil, fmt.Errorf("failed to draw captcha: %w", err)
	}

	if err := g.store.Set(ctx, id, answer); err != nil {
		return nil, fmt.Errorf("failed to store captcha: %w", err)
	}

	return &CaptchaResponse{
		CaptchaID: id,
		ImageData: item.EncodeB64string(),
	}, nil
}

func (g *SecureDigitGenerator) Verify(ctx context.Context, id string, answer string) bool {
	if g == nil || g.store == nil {
		return false
	}
	return g.store.Verify(ctx, strings.TrimSpace(id), strings.TrimSpace(answer), true)
}

func cryptoDigits(length int) (string, error) {
	if length <= 0 {
		length = 6
	}
	var sb strings.Builder
	sb.Grow(length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		sb.WriteByte(byte('0' + n.Int64()))
	}
	return sb.String(), nil
}

func generateSecureKey(numBytes int) (string, error) {
	if numBytes <= 0 {
		numBytes = 32
	}
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// URL-safe, no padding.
	return base64.RawURLEncoding.EncodeToString(b), nil
}
