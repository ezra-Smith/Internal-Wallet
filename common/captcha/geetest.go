package captcha

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

const (
	defaultGeetestAPIServer      = "http://gcaptcha4.geetest.com"
	defaultGeetestTimeoutMillis  = int64(5000)
	geetestValidatePath          = "/validate"
	geetestValidateSuccessResult = "success"
)

// GeetestConfig holds config for Geetest v4 validation.
type GeetestConfig struct {
	Enabled    bool   `json:",optional"`
	CaptchaID  string `json:",optional"`
	CaptchaKey string `json:",optional"`
	APIServer  string `json:",optional"`
	Timeout    int64  `json:",optional"`
	FailOpen   bool   `json:",optional"`
}

// GeetestValidationResult provides detailed validation outcome.
type GeetestValidationResult struct {
	Success  bool
	FailOpen bool
	Reason   string
	Err      error
}

// GeetestClient validates Geetest v4 captcha tokens.
type GeetestClient struct {
	cfg        GeetestConfig
	httpClient *http.Client
}

// NewGeetestClient builds a GeetestClient with sane defaults.
func NewGeetestClient(cfg GeetestConfig) *GeetestClient {
	normalized := normalizeGeetestConfig(cfg)
	timeout := normalized.Timeout
	if timeout <= 0 {
		timeout = defaultGeetestTimeoutMillis
	}
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Millisecond,
	}
	return &GeetestClient{
		cfg:        normalized,
		httpClient: client,
	}
}

// ValidateGeetest validates Geetest v4 parameters (simple signature).
func (c *GeetestClient) ValidateGeetest(ctx context.Context, lotNumber, captchaOutput, passToken, genTime string) (bool, error) {
	result := c.ValidateGeetestDetailed(ctx, lotNumber, captchaOutput, passToken, genTime)
	return result.Success, result.Err
}

// ValidateGeetestDetailed validates Geetest v4 parameters and returns detailed result.
func (c *GeetestClient) ValidateGeetestDetailed(ctx context.Context, lotNumber, captchaOutput, passToken, genTime string) GeetestValidationResult {
	if c == nil {
		return GeetestValidationResult{Success: true, FailOpen: true, Reason: "client_nil", Err: errors.New("geetest client is nil")}
	}
	if !c.cfg.Enabled {
		return GeetestValidationResult{Success: true, Reason: "geetest_disabled"}
	}

	lotNumber = strings.TrimSpace(lotNumber)
	captchaOutput = strings.TrimSpace(captchaOutput)
	passToken = strings.TrimSpace(passToken)
	genTime = strings.TrimSpace(genTime)

	if lotNumber == "" || captchaOutput == "" || passToken == "" || genTime == "" {
		return GeetestValidationResult{Success: false, Reason: "missing_params", Err: errors.New("geetest params required")}
	}

	if c.cfg.CaptchaID == "" || c.cfg.CaptchaKey == "" {
		return c.failOpen("missing_config", errors.New("geetest captcha_id/captcha_key missing"))
	}

	apiServer := strings.TrimRight(c.cfg.APIServer, "/")
	if apiServer == "" {
		apiServer = defaultGeetestAPIServer
	}
	endpoint := apiServer + geetestValidatePath + "?captcha_id=" + url.QueryEscape(c.cfg.CaptchaID)

	form := url.Values{}
	form.Set("lot_number", lotNumber)
	form.Set("captcha_output", captchaOutput)
	form.Set("pass_token", passToken)
	form.Set("gen_time", genTime)
	form.Set("sign_token", GenerateSignToken(c.cfg.CaptchaKey, lotNumber))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return c.failOpen("request_build_failed", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return c.failOpen("request_failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.failOpen("bad_status", errors.New(resp.Status))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.failOpen("read_failed", err)
	}

	var parsed geetestValidateResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return c.failOpen("invalid_response", err)
	}

	if strings.EqualFold(strings.TrimSpace(parsed.Result), geetestValidateSuccessResult) {
		return GeetestValidationResult{Success: true, Reason: parsed.Reason}
	}

	return GeetestValidationResult{Success: false, Reason: parsed.Reason}
}

func (c *GeetestClient) failOpen(reason string, err error) GeetestValidationResult {
	if c.cfg.FailOpen {
		return GeetestValidationResult{
			Success:  true,
			FailOpen: true,
			Reason:   reason,
			Err:      err,
		}
	}
	return GeetestValidationResult{Success: false, Reason: reason, Err: err}
}

type geetestValidateResponse struct {
	Result      string          `json:"result"`
	Reason      string          `json:"reason"`
	CaptchaArgs json.RawMessage `json:"captcha_args"`
}

// GenerateSignToken generates HMAC-SHA256 sign_token using captchaKey and lotNumber.
func GenerateSignToken(captchaKey, lotNumber string) string {
	mac := hmac.New(sha256.New, []byte(captchaKey))
	mac.Write([]byte(lotNumber))
	return hex.EncodeToString(mac.Sum(nil))
}

func normalizeGeetestConfig(cfg GeetestConfig) GeetestConfig {
	cfg.CaptchaID = normalizePlaceholder(cfg.CaptchaID)
	cfg.CaptchaKey = normalizePlaceholder(cfg.CaptchaKey)
	cfg.APIServer = normalizePlaceholder(cfg.APIServer)
	return cfg
}

func normalizePlaceholder(value string) string {
	v := strings.TrimSpace(value)
	if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
		return ""
	}
	return v
}

var defaultGeetestClient atomic.Value

// SetDefaultGeetestClient sets the package-level Geetest client.
func SetDefaultGeetestClient(client *GeetestClient) {
	defaultGeetestClient.Store(client)
}

// ValidateGeetest validates captcha parameters with the default client (if set).
func ValidateGeetest(ctx context.Context, lotNumber, captchaOutput, passToken, genTime string) (bool, error) {
	if c, ok := defaultGeetestClient.Load().(*GeetestClient); ok && c != nil {
		return c.ValidateGeetest(ctx, lotNumber, captchaOutput, passToken, genTime)
	}
	return false, errors.New("geetest client not initialized")
}
