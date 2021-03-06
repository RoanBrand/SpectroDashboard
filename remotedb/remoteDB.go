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

func SetupRemoteDB(conf *config.Config) {
	c := &conf.RemoteDatabase
	connString = fmt.Sprintf("server=%s;user id=%s;password=%s;database=%s", c.Address, c.User, c.Password, c.Database)
	table = c.Table
}

// Insert new results from spectro machines into remote MS SQL Server database.
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
	if err = tx.QueryRow(`SELECT TOP (1) DateTimeStamp FROM ` + table + ` ORDER BY ID DESC;`).Scan(&lastTime); err != nil {
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
		qry.WriteString(" (DateTimeStamp, SampleName, Furname, Spectro, ")
		for j, r := range s.Results {
			qry.WriteString(r.Element)
			if j < len(s.Results)-1 {
				qry.WriteString(", ")
			}
		}
		qry.WriteString(") VALUES ('")
		ist := s.TimeStamp.Format("2006-01-02 15:04:05")
		qry.WriteString(ist)
		qry.WriteString("', '")
		qry.WriteString(s.SampleName)
		qry.WriteString("', '")
		qry.WriteString(s.Furnace)
		qry.WriteString("', ")
		qry.WriteString(strconv.Itoa(s.Spectro))
		qry.WriteString(", ")
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
