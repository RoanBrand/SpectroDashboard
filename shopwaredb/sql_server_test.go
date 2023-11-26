package shopwaredb_test

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
)

func TestNewDriverAndWay(t *testing.T) {
	err := do()
	if err != nil {
		t.Fatal(err)
	}
}

func do() error {
	connString := "server=localhost;user id=Roan;password=qqaazz;database=Shopware;encrypt=disable"

	db, err := sql.Open("mssql", connString)
	if err != nil {
		return err
	}

	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	var lastTime time.Time
	if err = tx.QueryRow(`SELECT TOP (1) DateTimeStamp FROM AccSpectrograph ORDER BY ID DESC;`).Scan(&lastTime); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			tx.Rollback()
			return err
		}
	}

	fmt.Println(lastTime)
	return nil
}
