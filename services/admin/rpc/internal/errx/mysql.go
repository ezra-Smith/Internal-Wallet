package errx

import (
	"errors"
	"strings"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

// MySQLTableNotFound detects MySQL error 1146 (table doesn't exist) and returns the missing table name.
func MySQLTableNotFound(err error) (table string, ok bool) {
	if err == nil {
		return "", false
	}

	var mysqlErr *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlErr) {
		return "", false
	}
	if mysqlErr.Number != 1146 {
		return "", false
	}

	// Typical message:
	//   "Table 'crypto_wallet.admin_menu' doesn't exist"
	msg := mysqlErr.Message
	start := strings.Index(msg, "Table '")
	if start == -1 {
		return "", true
	}
	start += len("Table '")
	end := strings.Index(msg[start:], "'")
	if end == -1 {
		return "", true
	}

	full := msg[start : start+end] // db.table or just table
	if dot := strings.LastIndex(full, "."); dot >= 0 && dot+1 < len(full) {
		return full[dot+1:], true
	}
	return full, true
}

// MySQLColumnNotFound detects MySQL error 1054 (unknown column) and returns the missing column name.
func MySQLColumnNotFound(err error) (column string, ok bool) {
	if err == nil {
		return "", false
	}

	var mysqlErr *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlErr) {
		return "", false
	}
	if mysqlErr.Number != 1054 {
		return "", false
	}

	// Typical message:
	//   "Unknown column 'two_factor_pending_secret' in 'field list'"
	msg := mysqlErr.Message
	start := strings.Index(msg, "Unknown column '")
	if start == -1 {
		return "", true
	}
	start += len("Unknown column '")
	end := strings.Index(msg[start:], "'")
	if end == -1 {
		return "", true
	}
	return msg[start : start+end], true
}
