package repository

import (
	"errors"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

func isDuplicateKeyError(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) {
		// 1062: ER_DUP_ENTRY
		return mysqlErr.Number == 1062
	}
	return false
}
