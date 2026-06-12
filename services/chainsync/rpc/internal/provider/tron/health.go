package tron

import "context"

func (t *TronProvider) HealthCheck(ctx context.Context) error {
	_, err := t.GetLatestBlock(ctx)
	if err != nil {
		t.recordRequest(false, 0)
		return err
	}
	t.recordRequest(true, 0)
	return nil
}

