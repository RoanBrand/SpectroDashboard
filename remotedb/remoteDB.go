package remotedb

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/RoanBrand/SpectroDashboard/config"
	"github.com/RoanBrand/SpectroDashboard/sample"
	_ "github.com/denisenkom/go-mssqldb"
)

var connString, table string

// last inserted sample
var timestamp time.Time

func SetupRemoteDB(conf *config.Config) {
	c := &conf.RemoteDatabase
	connString = fmt.Sprintf("server=%s\\%s;user id=%s;password=%s;database=%s", c.Address, c.ServerName, c.User, c.Password, c.Database)
	table = c.Table
}

func getLastInsertRemoteDB() error {
	conn, err := sql.Open("mssql", connString)
	if err != nil {
		return err
	}

	defer conn.Close()

	err = conn.QueryRow(`SELECT DateTimeStamp FROM ` + table + ` ORDER BY ID DESC LIMIT 1;`).Scan(&timestamp)
	if err != nil {
		if err != sql.ErrNoRows {
			return err
		}
	}

	return nil
}

func InsertNewResultsRemoteDB(samples []sample.Record) error {
	conn, err := sql.Open("mssql", connString)
	if err != nil {
		return err
	}

	defer conn.Close()

	tx, err := conn.Begin()
	if err != nil {
		return err
	}

	var lastTime time.Time
	if err = tx.QueryRow(`SELECT DateTimeStamp FROM ` + table + ` ORDER BY ID DESC LIMIT 1;`).Scan(&timestamp); err != nil {
		if err != sql.ErrNoRows {
			tx.Rollback()
			return err
		}
	}

	for i := len(samples) - 1; i >= 0; i-- {
		s := &samples[i]
		if s.TimeStamp.Before(lastTime) {
			continue
		}

		qry := strings.Builder{}
		qry.WriteString("INSERT INTO ")
		qry.WriteString(table)
		qry.WriteString(" (")
		for j, r := range s.Results {
			qry.WriteString(r.Element)
			if j < len(s.Results)-1 {
				qry.WriteString(", ")
			}
		}
		qry.WriteString(") VALUES (")
		for j, r := range s.Results {
			qry.WriteString(strconv.FormatFloat(r.Value, 'f', 8, 64))
			if j < len(s.Results)-1 {
				qry.WriteString(", ")
			}
		}
		qry.WriteString(");")

		if _, err := tx.Exec(qry.String()); err != nil {
			tx.Rollback()
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		tx.Rollback()
		return err
	}

	return nil
}
