package logic

import (
	"errors"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

func isMySQLDuplicate(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
