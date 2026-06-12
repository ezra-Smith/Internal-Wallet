package logic

import (
	"context"
	"testing"

	"internalwallet/proto/pb"

	"github.com/stretchr/testify/assert"
)

func TestHealthCheckLogic_HealthCheck(t *testing.T) {
	ctx := requireSignerSvcCtx(t)

	logic := NewHealthCheckLogic(context.Background(), ctx)
	resp, err := logic.HealthCheck(&pb.HealthCheckRequest{})

	assert.NoError(t, err)
	assert.Equal(t, int32(0), resp.Code)
	assert.NotEmpty(t, resp.Status)
	assert.NotEmpty(t, resp.Timestamp)

	t.Logf("Health Check: Status=%s, Timestamp=%s", resp.Status, resp.Timestamp)
}
