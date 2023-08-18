package shopwaredb

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/RoanBrand/SpectroDashboard/config"
	"github.com/RoanBrand/SpectroDashboard/log"
	"github.com/RoanBrand/SpectroDashboard/sample"
	_ "github.com/denisenkom/go-mssqldb"
)

var connString, table string

func SetupShopwareDB(conf *config.Config) {
	c := &conf.ShopwareDB
	connString = fmt.Sprintf("server=%s;user id=%s;password=%s;database=%s", c.Address, c.User, c.Password, c.Database)
	table = c.Table
}

// Insert new results from spectro machines into foundry's Shopware MS SQL Server database.
func InsertNewResults(samples []sample.Record, debug bool) error {
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

	// We insert wall time (without TZ), so DB returns as UTC. Convert here to SAST, preserving wall clock time.
	lastTime, err = time.ParseInLocation("2006-01-02 15:04:05", lastTime.Format("2006-01-02 15:04:05"), time.Local)
	if err != nil {
		return err
	}

	if debug {
		log.Printf("remote DB last sample timestamp: %s\n", lastTime)
	}

	for i := len(samples) - 1; i >= 0; i-- {
		s := &samples[i]
		if !s.TimeStamp.After(lastTime) {
			continue
		}

		qry := strings.Builder{}
		qry.WriteString(`INSERT INTO "`)
		qry.WriteString(table)
		qry.WriteString(`" ("DateTimeStamp", "SampleName", "Furname", "Spectro"`)
		for _, r := range s.Results {
			if r.Element == "" {
				continue
			}

			qry.WriteString(`, "`)
			qry.WriteString(r.Element)
			qry.WriteByte('"')
		}
		qry.WriteString(`) VALUES ('`)
		// TODO: check if DB columb can store timezone, and if so, insert raw as query param.
		qry.WriteString(s.TimeStamp.Format("2006-01-02 15:04:05"))
		qry.WriteString("', '")
		qry.WriteString(s.SampleName)
		qry.WriteString("', '")
		qry.WriteString(s.Furnace)
		qry.WriteString("', ")
		qry.WriteString(strconv.Itoa(s.Spectro))
		for _, r := range s.Results {
			if r.Element == "" {
				continue
			}

			qry.WriteString(`, `)
			qry.WriteString(strconv.FormatFloat(r.Value, 'f', 8, 64))
		}
		qry.WriteString(");")

		q := qry.String()
		if debug {
			log.Println("remote DB query: " + q)
		}
		if _, err := tx.Exec(q); err != nil {
			tx.Rollback()
			return errors.New("error executing insert statement: " + q + " Error: " + err.Error())
		}
	}

	if err = tx.Commit(); err != nil {
		tx.Rollback()
		return err
	}

	return nil
}
