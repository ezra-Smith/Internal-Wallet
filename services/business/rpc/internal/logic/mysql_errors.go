package logic

import (
	"errors"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

func mySQLErrno(err error) (uint16, bool) {
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr != nil {
		return mysqlErr.Number, true
	}
	return 0, false
}

func isMySQLDuplicateKey(err error) bool {
	no, ok := mySQLErrno(err)
	return ok && no == 1062
}

// isMySQLNoDefaultValue checks for:
// Error 1364 (HY000): Field 'xxx' doesn't have a default value
func isMySQLNoDefaultValue(err error) bool {
	no, ok := mySQLErrno(err)
	return ok && no == 1364
}
