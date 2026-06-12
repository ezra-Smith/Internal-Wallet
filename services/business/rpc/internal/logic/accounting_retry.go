package logic

import (
	"context"
	"time"

	"internalwallet/proto/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func callAccountingTxWithRetry(ctx context.Context, maxAttempts int, fn func(context.Context) (*pb.LedgerTxResponse, error)) (*pb.LedgerTxResponse, error) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	var lastResp *pb.LedgerTxResponse
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		resp, err := fn(ctx)
		lastResp = resp
		lastErr = err
		if err == nil && resp != nil && resp.Success {
			return resp, nil
		}

		// Only retry transient RPC errors.
		if err != nil {
			if st, ok := status.FromError(err); ok {
				switch st.Code() {
				case codes.Unavailable, codes.DeadlineExceeded:
					// retry
				default:
					return resp, err
				}
			} else {
				return resp, err
			}
		} else if resp != nil && !resp.Success {
			// Business failure is not retryable here.
			return resp, nil
		}

		// Backoff (bounded).
		backoff := time.Duration(attempt+1) * 200 * time.Millisecond
		if backoff > 2*time.Second {
			backoff = 2 * time.Second
		}
		select {
		case <-ctx.Done():
			return lastResp, lastErr
		case <-time.After(backoff):
		}
	}
	return lastResp, lastErr
}
