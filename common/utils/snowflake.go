package utils

import (
	"fmt"
	"strconv"
	"sync"
	"time"
)

// Snowflake ID生成器
// 用于生成全局唯一的分布式ID
// ID结构：41位时间戳 + 10位机器ID + 12位序列号 = 63位
type Snowflake struct {
	mu        sync.Mutex
	timestamp int64
	sequence  int64
	machineID int64
}

const (
	epoch              = int64(1609459200000)                     // 2021-01-01 00:00:00 UTC (自定义起始时间)
	machineIDBits      = uint(10)                                 // 机器ID位数（最多1024台机器）
	sequenceBits       = uint(12)                                 // 序列号位数（每毫秒最多4096个ID）
	maxMachineID       = int64(-1) ^ (int64(-1) << machineIDBits) // 最大机器ID: 1023
	maxSequence        = int64(-1) ^ (int64(-1) << sequenceBits)  // 最大序列号: 4095
	timestampLeftShift = machineIDBits + sequenceBits             // 时间戳左移22位
	machineIDLeftShift = sequenceBits                             // 机器ID左移12位
)

var (
	defaultSnowflake *Snowflake
	once             sync.Once
)

// Init 初始化全局雪花ID生成器（每个服务启动时调用一次）
// machineID: 机器/服务ID，范围 0-1023，必须保证全局唯一
// 建议配置：
//   - account-rpc: 1
//   - wallet-rpc:  2
//   - trade-rpc:   3
//   - market-rpc:  4
//   - settlement-rpc: 5
//   - risk-rpc:    6
//   - notification-rpc: 7
func Init(machineID int64) error {
	var err error
	once.Do(func() {
		if machineID < 0 || machineID > maxMachineID {
			err = fmt.Errorf("machineID must be between 0 and %d, got %d", maxMachineID, machineID)
			return
		}
		defaultSnowflake = NewSnowflake(machineID)
	})
	return err
}

// NewSnowflake 创建Snowflake实例
func NewSnowflake(machineID int64) *Snowflake {
	if machineID < 0 || machineID > maxMachineID {
		panic(fmt.Sprintf("machineID must be between 0 and %d", maxMachineID))
	}
	return &Snowflake{
		machineID: machineID,
		sequence:  0,
		timestamp: 0,
	}
}

// NextID 生成下一个ID（返回 int64）
func (s *Snowflake) NextID() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()

	// 时钟回拨检测
	if now < s.timestamp {
		panic(fmt.Sprintf("clock moved backwards, refusing to generate id for %d milliseconds", s.timestamp-now))
	}

	if now == s.timestamp {
		// 同一毫秒内，序列号递增
		s.sequence = (s.sequence + 1) & maxSequence
		if s.sequence == 0 {
			// 序列号溢出，等待下一毫秒
			for now <= s.timestamp {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		// 新的毫秒，序列号重置
		s.sequence = 0
	}

	s.timestamp = now

	// 组装ID：时间戳 | 机器ID | 序列号
	id := ((now - epoch) << timestampLeftShift) |
		(s.machineID << machineIDLeftShift) |
		s.sequence

	return id
}

// NextIDString 生成下一个ID（返回 string）
// 金融系统推荐使用 string 类型，避免 JavaScript 精度问题
func (s *Snowflake) NextIDString() string {
	return strconv.FormatInt(s.NextID(), 10)
}

// GenerateID 生成唯一ID（int64格式）
// 使用前必须先调用 Init() 初始化
func GenerateID() int64 {
	if defaultSnowflake == nil {
		panic("snowflake not initialized, please call Init() first")
	}
	return defaultSnowflake.NextID()
}

// GenerateIDString 生成唯一ID（string格式，推荐）
// 使用前必须先调用 Init() 初始化
func GenerateIDString() string {
	if defaultSnowflake == nil {
		panic("snowflake not initialized, please call Init() first")
	}
	return defaultSnowflake.NextIDString()
}

// GetMachineID 获取当前机器ID
func GetMachineID() int64 {
	if defaultSnowflake == nil {
		return -1
	}
	return defaultSnowflake.machineID
}
