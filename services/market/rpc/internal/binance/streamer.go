package binance

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"internalwallet/services/market/rpc/internal/config"
	"internalwallet/services/market/rpc/internal/status"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

var errMaxConnAge = errors.New("max websocket connection age reached")

type MiniTickerStreamer struct {
	cfg       config.Config
	redis     *redis.Client
	status    *status.ServiceStatus
	sparkline *SparklineCollector
}

func NewMiniTickerStreamer(cfg config.Config, redisClient *redis.Client, st *status.ServiceStatus, sparkline *SparklineCollector) *MiniTickerStreamer {
	return &MiniTickerStreamer{
		cfg:       cfg,
		redis:     redisClient,
		status:    st,
		sparkline: sparkline,
	}
}

func (s *MiniTickerStreamer) Run(ctx context.Context) error {
	wsURL, err := s.cfg.WSURL()
	if err != nil {
		return err
	}

	b := newBackoff(s.cfg.Reconnect)

	for {
		if ctx.Err() != nil {
			return nil
		}

		err := s.runOnce(ctx, wsURL, b)
		if ctx.Err() != nil {
			return nil
		}
		if err == nil {
			continue
		}

		s.status.SetWSConnected(false)
		s.status.SetWSError(err)

		if errors.Is(err, errMaxConnAge) {
			logx.Infof("WebSocket max age reached, reconnecting...")
			b.Reset()
			continue
		}

		s.status.AddReconnect()
		delay := b.NextDelay()
		logx.Errorf("WebSocket disconnected: %v (reconnect in %s)", err, delay)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *MiniTickerStreamer) runOnce(ctx context.Context, wsURL string, b *backoff) error {
	handshakeTimeout := time.Duration(s.cfg.Binance.HandshakeTimeoutMillis) * time.Millisecond
	readTimeout := time.Duration(s.cfg.Binance.ReadTimeoutMillis) * time.Millisecond
	writeTimeout := time.Duration(s.cfg.Binance.WriteTimeoutMillis) * time.Millisecond
	maxConnAge := time.Duration(s.cfg.Binance.MaxConnAgeSeconds) * time.Second

	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: handshakeTimeout,
	}

	conn, resp, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("ws dial failed (status=%s): %w", resp.Status, err)
		}
		return fmt.Errorf("ws dial failed: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	// Successful dial: reset backoff so future disconnects start with the initial delay.
	if b != nil {
		b.Reset()
	}

	startedAt := time.Now()
	s.status.SetWSConnected(true)
	logx.Infof("WebSocket connected")

	conn.SetReadLimit(s.cfg.Binance.MaxMessageBytes)

	extendReadDeadline := func() {
		_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
	}
	extendReadDeadline()

	conn.SetPingHandler(func(appData string) error {
		// Binance expects a pong within 1 minute, payload must match exactly.
		extendReadDeadline()
		if time.Since(startedAt) >= maxConnAge {
			_ = conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "reconnect"),
				time.Now().Add(writeTimeout),
			)
			return errMaxConnAge
		}
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(writeTimeout))
	})
	conn.SetPongHandler(func(_ string) error {
		extendReadDeadline()
		return nil
	})

	ttl := s.cfg.RedisTTL()
	hashKey := s.cfg.Redis.TickerHashKey

	connCtx, cancel := context.WithCancel(ctx)
	payloadCh := make(chan []byte, 1)
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		s.redisWriter(connCtx, hashKey, ttl, payloadCh)
	}()
	defer func() {
		cancel()
		close(payloadCh)
		<-writerDone
	}()

	for {
		if ctx.Err() != nil {
			_ = conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "shutdown"),
				time.Now().Add(writeTimeout),
			)
			return nil
		}

		if time.Since(startedAt) >= maxConnAge {
			_ = conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "reconnect"),
				time.Now().Add(writeTimeout),
			)
			return errMaxConnAge
		}

		extendReadDeadline()
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		s.status.MarkWSReceive()

		// Never block the WS read loop on Redis writes; keep only the latest payload.
		select {
		case payloadCh <- msg:
		default:
			select {
			case <-payloadCh:
			default:
			}
			select {
			case payloadCh <- msg:
			default:
			}
		}
	}
}

func (s *MiniTickerStreamer) redisWriter(ctx context.Context, hashKey string, ttl time.Duration, payloadCh <-chan []byte) {
	var lastParseLog time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-payloadCh:
			if !ok {
				return
			}

			events, maxEventTime, err := parseMiniTickerPayload(msg)
			if err != nil {
				// Avoid log spam if Binance sends an unexpected payload repeatedly.
				if time.Since(lastParseLog) > 10*time.Second {
					logx.Errorf("Failed to parse miniTicker payload: %v", err)
					lastParseLog = time.Now()
				}
				continue
			}

			s.status.SetWSEventTime(maxEventTime)

			// Update sparkline collector with latest prices (for periodic sampling)
			if s.sparkline != nil {
				s.sparkline.UpdatePrices(events)
			}

			if err := s.store(ctx, hashKey, ttl, events); err != nil {
				if ctx.Err() != nil {
					return
				}
				s.status.SetRedisError(err)
				logx.Errorf("Redis write failed: %v", err)
				continue
			}

			s.status.MarkRedisWrite()
		}
	}
}

func (s *MiniTickerStreamer) store(ctx context.Context, hashKey string, ttl time.Duration, events []miniTickerEvent) error {
	if len(events) == 0 {
		return nil
	}

	updates := make(map[string]string, len(events))
	for _, e := range events {
		if len(e.Symbol) == 0 || len(e.Close) == 0 || e.EventTime <= 0 {
			continue
		}
		// Symbol may include UTF-8; Redis keys/fields are binary-safe, and Go strings are UTF-8.
		updates[e.Symbol] = encodeTickerValue(e.Close, e.EventTime)
	}
	if len(updates) == 0 {
		return nil
	}

	pipe := s.redis.TxPipeline()
	pipe.HSet(ctx, hashKey, updates)
	if ttl > 0 {
		pipe.Expire(ctx, hashKey, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}
