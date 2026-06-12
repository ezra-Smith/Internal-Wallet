package logic

import (
	"fmt"
	"time"
)

// parseBool safely converts various DB boolean representations to bool
// MySQL TINYINT(1) columns are returned as int64 by the driver, not bool
func parseBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case int64:
		return val != 0
	case int32:
		return val != 0
	case int16:
		return val != 0
	case int8:
		return val != 0
	case int:
		return val != 0
	case uint64:
		return val != 0
	case uint32:
		return val != 0
	case uint16:
		return val != 0
	case uint8:
		return val != 0
	case uint:
		return val != 0
	case []uint8:
		// byte slice (可能是字符串 "0" 或 "1")
		if len(val) == 0 {
			return false
		}
		return val[0] != '0' && val[0] != 0
	case string:
		// 字符串形式的布尔值
		return val != "" && val != "0" && val != "false" && val != "FALSE"
	default:
		// 如果遇到未知类型，记录日志并返回 false（安全默认值）
		// 这可以帮助我们发现新的数据类型
		fmt.Printf("parseBool: unexpected type %T for value %v\n", v, v)
		return false
	}
}

// parseInt64 safely converts various DB integer representations to int64
func parseInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int32:
		return int64(val)
	case int16:
		return int64(val)
	case int8:
		return int64(val)
	case int:
		return int64(val)
	case uint64:
		return int64(val)
	case uint32:
		return int64(val)
	case uint16:
		return int64(val)
	case uint8:
		return int64(val)
	case uint:
		return int64(val)
	default:
		fmt.Printf("parseInt64: unexpected type %T for value %v\n", v, v)
		return 0
	}
}

// parseString safely converts various DB types to string
// Handles DECIMAL ([]uint8), VARCHAR (string), DATETIME (time.Time)
func parseString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []uint8:
		// DECIMAL 类型返回为 []uint8
		return string(val)
	case time.Time:
		// DATETIME 类型，格式化为标准时间字符串
		return val.Format("2006-01-02 15:04:05")
	case nil:
		return ""
	default:
		// 其他类型转换为字符串
		return fmt.Sprintf("%v", val)
	}
}
