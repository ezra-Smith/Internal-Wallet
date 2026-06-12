package status

import (
	"sync/atomic"
	"time"
)

type Snapshot struct {
	WSConnected            bool  `json:"wsConnected"`
	LastWSMessageUnixMilli int64 `json:"lastWsMessageUnixMilli"`
	LastWSEventUnixMilli   int64 `json:"lastWsEventUnixMilli"`

	LastRedisWriteUnixMilli int64  `json:"lastRedisWriteUnixMilli"`
	LastWSError             string `json:"lastWsError,omitempty"`
	LastRedisError          string `json:"lastRedisError,omitempty"`
	Reconnects              int64  `json:"reconnects"`
}

type ServiceStatus struct {
	wsConnected atomic.Bool

	lastWSMessageUnixMilli atomic.Int64
	lastWSEventUnixMilli   atomic.Int64

	lastRedisWriteUnixMilli atomic.Int64

	reconnects atomic.Int64

	lastWSError    atomic.Value // string
	lastRedisError atomic.Value // string
}

func New() *ServiceStatus {
	s := &ServiceStatus{}
	s.lastWSError.Store("")
	s.lastRedisError.Store("")
	return s
}

func (s *ServiceStatus) SetWSConnected(v bool) {
	s.wsConnected.Store(v)
}

func (s *ServiceStatus) MarkWSReceive() {
	s.lastWSMessageUnixMilli.Store(time.Now().UnixMilli())
}

func (s *ServiceStatus) SetWSEventTime(eventTimeUnixMilli int64) {
	if eventTimeUnixMilli <= 0 {
		return
	}
	s.lastWSEventUnixMilli.Store(eventTimeUnixMilli)
}

func (s *ServiceStatus) MarkRedisWrite() {
	s.lastRedisWriteUnixMilli.Store(time.Now().UnixMilli())
}

func (s *ServiceStatus) AddReconnect() {
	s.reconnects.Add(1)
}

func (s *ServiceStatus) SetWSError(err error) {
	if err == nil {
		return
	}
	s.lastWSError.Store(err.Error())
}

func (s *ServiceStatus) SetRedisError(err error) {
	if err == nil {
		return
	}
	s.lastRedisError.Store(err.Error())
}

func (s *ServiceStatus) Snapshot() Snapshot {
	wsErr, _ := s.lastWSError.Load().(string)
	redisErr, _ := s.lastRedisError.Load().(string)
	return Snapshot{
		WSConnected:             s.wsConnected.Load(),
		LastWSMessageUnixMilli:  s.lastWSMessageUnixMilli.Load(),
		LastWSEventUnixMilli:    s.lastWSEventUnixMilli.Load(),
		LastRedisWriteUnixMilli: s.lastRedisWriteUnixMilli.Load(),
		LastWSError:             wsErr,
		LastRedisError:          redisErr,
		Reconnects:              s.reconnects.Load(),
	}
}
