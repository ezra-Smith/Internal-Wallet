package selfhosted

import (
	"context"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
)

func (p *SelfHostedProvider) HealthCheck(ctx context.Context) error {
	baseURL := strings.TrimRight(p.config.Endpoint, "/")
	switch p.config.Type {
	case "ethereum":
		baseURL = strings.TrimSuffix(baseURL, "/eth")
	case "bsc":
		baseURL = strings.TrimSuffix(baseURL, "/bsc")
	case "tron":
		baseURL = strings.TrimSuffix(baseURL, "/tron")
	}

	if err := testConnection(ctx, p.client, baseURL, p.config.Chain); err != nil {
		p.RecordRequest(false, 0)
		return fmt.Errorf("health check failed: %v", err)
	}

	p.RecordRequest(true, 0)
	logx.Debugf("Health check passed for provider %s", p.id)
	return nil
}

