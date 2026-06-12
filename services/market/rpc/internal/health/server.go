package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"internalwallet/services/market/rpc/internal/config"
	"internalwallet/services/market/rpc/internal/status"

	"github.com/redis/go-redis/v9"
)

type Server struct {
	cfg   config.HealthConfig
	redis *redis.Client
	st    *status.ServiceStatus

	httpServer *http.Server
}

func NewServer(cfg config.HealthConfig, redisClient *redis.Client, st *status.ServiceStatus) *Server {
	mux := http.NewServeMux()
	s := &Server{
		cfg:   cfg,
		redis: redisClient,
		st:    st,
		httpServer: &http.Server{
			Addr:              cfg.ListenOn,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}

	mux.HandleFunc(cfg.Path, s.handleHealth)
	return s
}

func (s *Server) Start() error {
	if !s.cfg.Enabled {
		return nil
	}
	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	snap := s.st.Snapshot()

	staleAfter := time.Duration(s.cfg.StaleAfterMillis) * time.Millisecond
	pingTimeout := time.Duration(s.cfg.RedisPingTimeoutMillis) * time.Millisecond

	wsFresh := snap.LastWSMessageUnixMilli > 0 && now.UnixMilli()-snap.LastWSMessageUnixMilli <= staleAfter.Milliseconds()
	wsOK := !s.cfg.RequireWS || (snap.WSConnected && wsFresh)

	redisOK := true
	if s.redis != nil {
		ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
		defer cancel()
		if err := s.redis.Ping(ctx).Err(); err != nil {
			redisOK = false
		}
	} else {
		redisOK = false
	}

	type resp struct {
		OK           bool            `json:"ok"`
		NowUnixMilli int64           `json:"nowUnixMilli"`
		WSOK         bool            `json:"wsOk"`
		RedisOK      bool            `json:"redisOk"`
		Status       status.Snapshot `json:"status"`
	}

	ok := wsOK && redisOK
	code := http.StatusOK
	if !ok {
		code = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)

	_ = json.NewEncoder(w).Encode(resp{
		OK:           ok,
		NowUnixMilli: now.UnixMilli(),
		WSOK:         wsOK,
		RedisOK:      redisOK,
		Status:       snap,
	})
}
